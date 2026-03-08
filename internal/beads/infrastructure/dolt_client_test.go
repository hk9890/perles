package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestResolveConnectionDetails(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3307)
	writeTestPortFile(t, beadsDir, "3311")

	details, err := ResolveConnectionDetails(beadsDir)
	require.NoError(t, err)
	require.Equal(t, "127.0.0.1", details.Host)
	require.Equal(t, 3311, details.Port)
	require.Equal(t, "perles", details.Database)
}

func TestResolveConnectionDetails_PortFallsBackToConfig(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 13849)

	details, err := ResolveConnectionDetails(beadsDir)
	require.NoError(t, err)
	require.Equal(t, 13849, details.Port)
}

func TestResolveConnectionDetails_UnsupportedBackend(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"sqlite","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 13849)

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsNoBeadsError(err))
	require.False(t, IsServerStartupError(err))
	require.Contains(t, err.Error(), "unsupported beads backend")
}

func TestResolveConnectionDetails_InvalidPortFile(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 13849)
	writeTestPortFile(t, beadsDir, "nope")

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsServerStartupError(err))
	require.False(t, IsNoBeadsError(err))
	require.Contains(t, err.Error(), "parsing dolt server port")

	var startupErr *StartupError
	require.True(t, errors.As(err, &startupErr))
	require.NotNil(t, startupErr.Details)
	require.Equal(t, "127.0.0.1", startupErr.Details.Host)
	require.Equal(t, 13849, startupErr.Details.Port)
	require.Equal(t, "perles", startupErr.Details.Database)
}

func TestResolveConnectionDetails_MissingMetadataClassifiedAsNoBeads(t *testing.T) {
	beadsDir := t.TempDir()

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsNoBeadsError(err))
	require.False(t, IsServerStartupError(err))
}

func TestResolveConnectionDetails_InvalidConfigClassifiedAsNoBeads(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	require.NoError(t, os.MkdirAll(filepath.Join(beadsDir, "dolt"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(beadsDir, "dolt", "config.yaml"), []byte("listener: ["), 0644))

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsNoBeadsError(err))
	require.False(t, IsServerStartupError(err))
}

func TestStartupErrorPredicatesUseErrorsAs(t *testing.T) {
	err := fmt.Errorf("outer: %w", &StartupError{Kind: StartupErrorKindNoBeads, Err: errors.New("boom")})
	require.True(t, IsNoBeadsError(err))
	require.False(t, IsServerStartupError(err))

	serverErr := fmt.Errorf("outer: %w", &StartupError{Kind: StartupErrorKindServerStartup, Err: errors.New("boom")})
	require.True(t, IsServerStartupError(serverErr))
	require.False(t, IsNoBeadsError(serverErr))

	plain := errors.New("plain")
	require.False(t, IsNoBeadsError(plain))
	require.False(t, IsServerStartupError(plain))
}

func TestDoltClientFailsWhenServerUnavailableWithConnectionDetails(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "192.0.2.10", 3306)

	originalConnect := connectDoltClient
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     errors.New("simulated unavailable server"),
			Details: copyConnectionDetails(details),
		}
	}
	t.Cleanup(func() { connectDoltClient = originalConnect })

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)
	require.True(t, IsServerStartupError(err))
	require.False(t, IsNoBeadsError(err))
	require.Contains(t, err.Error(), "simulated unavailable server")

	var startupErr *StartupError
	require.True(t, errors.As(err, &startupErr))
	require.NotNil(t, startupErr.Details)
	require.Equal(t, "192.0.2.10", startupErr.Details.Host)
	require.Equal(t, 3306, startupErr.Details.Port)
	require.Equal(t, "perles", startupErr.Details.Database)
}

func TestNewDoltClient_DelegatesStartupAndRetriesWithReresolvedPort(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	originalBackoff := postStartReadinessBackoff
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
		postStartReadinessBackoff = originalBackoff
	})

	postStartReadinessBackoff = nil

	var connectCalls []ConnectionDetails
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls = append(connectCalls, details)
		if len(connectCalls) == 1 {
			return nil, &StartupError{
				Kind:    StartupErrorKindServerStartup,
				Err:     errors.New("initial dial refused"),
				Details: copyConnectionDetails(details),
			}
		}
		return &DoltClient{details: details}, nil
	}

	delegateCalled := false
	newDoltStartupDelegate = func(resolvedBeadsDir string) doltStartupDelegate {
		require.Equal(t, beadsDir, resolvedBeadsDir)
		return &fakeDoltStartupDelegate{startFunc: func(_ context.Context) error {
			delegateCalled = true
			writeTestPortFile(t, beadsDir, "4020")
			return nil
		}}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.True(t, delegateCalled)
	require.Len(t, connectCalls, 2)
	require.Equal(t, 4010, connectCalls[0].Port)
	require.Equal(t, 4020, connectCalls[1].Port)
	require.Equal(t, 4020, client.details.Port)
}

func TestNewDoltClient_DoesNotDelegateOnRemoteHost(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "10.23.45.67", 3306)

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
	})

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     errors.New("server unreachable"),
			Details: copyConnectionDetails(details),
		}
	}

	delegateCalled := false
	newDoltStartupDelegate = func(_ string) doltStartupDelegate {
		delegateCalled = true
		return &fakeDoltStartupDelegate{startFunc: func(_ context.Context) error { return nil }}
	}

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)
	require.True(t, IsServerStartupError(err))
	require.Equal(t, 1, connectCalls)
	require.False(t, delegateCalled)
}

func TestNewDoltClient_MissingBDReturnsActionableError(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "localhost", 3306)

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
	})

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     errors.New("cannot reach local dolt server"),
			Details: copyConnectionDetails(details),
		}
	}

	newDoltStartupDelegate = func(resolvedBeadsDir string) doltStartupDelegate {
		return &bdDoltStarter{
			beadsDir: resolvedBeadsDir,
			timeout:  50 * time.Millisecond,
			lookPathFunc: func(_ string) (string, error) {
				return "", exec.ErrNotFound
			},
			runFunc: runBeadsCommand,
		}
	}

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)
	require.True(t, IsServerStartupError(err))
	require.Contains(t, err.Error(), "requires 'bd' CLI in PATH")
}

func TestNewDoltClient_DelegatedStartFailureIncludesOutput(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
	})

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     errors.New("server down"),
			Details: copyConnectionDetails(details),
		}
	}

	newDoltStartupDelegate = func(resolvedBeadsDir string) doltStartupDelegate {
		return &bdDoltStarter{
			beadsDir: resolvedBeadsDir,
			timeout:  50 * time.Millisecond,
			lookPathFunc: func(_ string) (string, error) {
				return "bd", nil
			},
			runFunc: func(_ context.Context, _ string, _ []string, _ []string) (string, string, error) {
				return "startup output", "cannot bind port", errors.New("exit status 1")
			},
		}
	}

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)
	require.True(t, IsServerStartupError(err))
	require.Contains(t, err.Error(), "delegated 'bd dolt start' failed")
	require.Contains(t, err.Error(), "stdout: startup output")
	require.Contains(t, err.Error(), "stderr: cannot bind port")
	require.Equal(t, 1, connectCalls)
}

func TestBDDoltStarter_StartSetsBeadsDirEnv(t *testing.T) {
	beadsDir := t.TempDir()
	starter := &bdDoltStarter{
		beadsDir: beadsDir,
		timeout:  50 * time.Millisecond,
		lookPathFunc: func(_ string) (string, error) {
			return "bd", nil
		},
		runFunc: func(_ context.Context, command string, args []string, env []string) (string, string, error) {
			require.Equal(t, "bd", command)
			require.Equal(t, []string{"dolt", "start"}, args)

			foundBeadsDir := false
			for _, kv := range env {
				if strings.HasPrefix(kv, "BEADS_DIR=") {
					require.Equal(t, "BEADS_DIR="+beadsDir, kv)
					foundBeadsDir = true
				}
			}
			require.True(t, foundBeadsDir)

			return "started", "", nil
		},
	}

	require.NoError(t, starter.Start(context.Background()))
}

func TestDoltClientDBAccessor(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	client := &DoltClient{db: db}
	require.NotNil(t, client.DB())
}

func TestDoltClientVersionReadsMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	rows := sqlmock.NewRows([]string{"value"}).AddRow("0.59.0")
	mock.ExpectQuery("SELECT value FROM metadata WHERE `key` = \\\\?").
		WithArgs("bd_version").
		WillReturnRows(rows)

	client := &DoltClient{db: db}
	version, err := client.Version()
	require.NoError(t, err)
	require.Equal(t, "0.59.0", version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltClientGetCommentsReadsRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ts1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	ts2 := ts1.Add(time.Minute)
	rows := sqlmock.NewRows([]string{"id", "author", "text", "created_at"}).
		AddRow(1, "alice", "first", ts1).
		AddRow(2, "bob", "second", ts2)

	mock.ExpectQuery("FROM comments").WithArgs("perles-1").WillReturnRows(rows)

	client := &DoltClient{db: db}
	comments, err := client.GetComments("perles-1")
	require.NoError(t, err)
	require.Len(t, comments, 2)
	require.Equal(t, 1, comments[0].ID)
	require.Equal(t, "bob", comments[1].Author)
	require.NoError(t, mock.ExpectationsWereMet())
}

func writeTestMetadata(t *testing.T, beadsDir, content string) {
	t.Helper()
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	require.NoError(t, os.WriteFile(metadataPath, []byte(content), 0644))
}

func writeTestDoltConfig(t *testing.T, beadsDir, host string, port int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(beadsDir, "dolt"), 0755))
	configPath := filepath.Join(beadsDir, "dolt", "config.yaml")
	content := []byte("listener:\n  host: " + host + "\n  port: " + strconv.Itoa(port) + "\n")
	require.NoError(t, os.WriteFile(configPath, content, 0644))
}

func writeTestPortFile(t *testing.T, beadsDir, content string) {
	t.Helper()
	portPath := filepath.Join(beadsDir, "dolt-server.port")
	require.NoError(t, os.WriteFile(portPath, []byte(content), 0644))
}

type fakeDoltStartupDelegate struct {
	startFunc func(ctx context.Context) error
}

func (f *fakeDoltStartupDelegate) Start(ctx context.Context) error {
	if f.startFunc != nil {
		return f.startFunc(ctx)
	}

	return nil
}
