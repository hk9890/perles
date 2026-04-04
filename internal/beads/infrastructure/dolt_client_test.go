package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysql "github.com/go-sql-driver/mysql"
	appbeads "github.com/hk9890/perles/internal/beads/application"
	"github.com/hk9890/perles/internal/log"
	"github.com/hk9890/perles/internal/pubsub"
	"github.com/hk9890/perles/internal/testutil"
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
	require.Equal(t, "Perles supports beads v1+ projects backed by Dolt. Run 'bd bootstrap' in a Dolt-backed beads project.", StartupSuggestion(err))
}

func TestResolveConnectionDetails_EmbeddedModeClassifiedAsNoBeads(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"embedded","dolt_database":"perlesv1spec"}`)

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsNoBeadsError(err))
	require.False(t, IsServerStartupError(err))
	require.Contains(t, err.Error(), `unsupported dolt mode "embedded"`)
	require.Contains(t, StartupSuggestion(err), "supports beads v1+ only in dolt_mode=server")
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
	originalRetryPolicyFactory := startupRetryPolicyFactory
	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
		startupRetryPolicyFactory = originalRetryPolicyFactory
		startupSleepWithContext = originalSleepWithContext
	})

	startupRetryPolicyFactory = func() RetryPolicy {
		return RetryPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: 0}
	}
	startupSleepWithContext = func(context.Context, time.Duration) error { return nil }

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
	require.Equal(t, "1", comments[0].ID)
	require.Equal(t, "2", comments[1].ID)
	require.Equal(t, "bob", comments[1].Author)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltClientGetCommentsReadsUUIDBackedRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	uuid := "019cfd8a-be96-774b-b1d6-bd02aef1880a"
	rows := sqlmock.NewRows([]string{"id", "author", "text", "created_at"}).
		AddRow([]byte(uuid), "alice", "first", ts)

	mock.ExpectQuery("FROM comments").WithArgs("perles-uuid").WillReturnRows(rows)

	client := &DoltClient{db: db}
	comments, err := client.GetComments("perles-uuid")
	require.NoError(t, err)
	require.Len(t, comments, 1)
	require.Equal(t, uuid, comments[0].ID)
	require.Equal(t, "alice", comments[0].Author)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDoltClientReconnect_ReReadsPortAndClosesOldDB(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	oldDB, _, err := sqlmock.New()
	require.NoError(t, err)
	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = newDB.Close() }()

	originalConnect := connectDoltClient
	t.Cleanup(func() { connectDoltClient = originalConnect })

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		switch connectCalls {
		case 1:
			require.Equal(t, 4010, details.Port)
			return &DoltClient{db: oldDB, details: details, beadsDir: beadsDir}, nil
		case 2:
			require.Equal(t, 4020, details.Port)
			return &DoltClient{db: newDB, details: details, beadsDir: beadsDir}, nil
		default:
			return nil, fmt.Errorf("unexpected connect call: %d", connectCalls)
		}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	writeTestPortFile(t, beadsDir, "4020")
	attempted, reconnectErr := client.ReconnectIfRecoverable(errors.New("dial tcp 127.0.0.1:4020: connect: connection refused"))
	require.True(t, attempted)
	require.NoError(t, reconnectErr)

	require.Equal(t, 2, connectCalls)
	require.Equal(t, newDB, client.DB())
	require.Equal(t, 4020, client.details.Port)

	// Verify old handle was closed during swap.
	require.Error(t, oldDB.Ping())
}

func TestDoltClientReconnect_LocalOnlyDelegatedRestart(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	firstDB, _, err := sqlmock.New()
	require.NoError(t, err)
	secondDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = firstDB.Close()
		_ = secondDB.Close()
	}()

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	originalRetryPolicyFactory := startupRetryPolicyFactory
	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
		startupRetryPolicyFactory = originalRetryPolicyFactory
		startupSleepWithContext = originalSleepWithContext
	})

	startupRetryPolicyFactory = func() RetryPolicy {
		return RetryPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: 0}
	}
	startupSleepWithContext = func(context.Context, time.Duration) error { return nil }

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		if connectCalls == 1 {
			return &DoltClient{db: firstDB, details: details, beadsDir: beadsDir}, nil
		}
		if connectCalls == 2 {
			return nil, &StartupError{Kind: StartupErrorKindServerStartup, Err: errors.New("connection refused"), Details: copyConnectionDetails(details)}
		}
		return &DoltClient{db: secondDB, details: details, beadsDir: beadsDir}, nil
	}

	delegateCalls := 0
	newDoltStartupDelegate = func(_ string) doltStartupDelegate {
		return &fakeDoltStartupDelegate{startFunc: func(_ context.Context) error {
			delegateCalls++
			return nil
		}}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	attempted, reconnectErr := client.ReconnectIfRecoverable(errors.New("dial tcp 127.0.0.1:4010: connect: connection refused"))
	require.True(t, attempted)
	require.NoError(t, reconnectErr)
	require.Equal(t, 1, delegateCalls)
	require.Equal(t, secondDB, client.DB())
}

func TestDoltClientReconnect_RemoteHostDoesNotDelegate(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "10.20.30.40", 3306)

	firstDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = firstDB.Close() }()

	originalConnect := connectDoltClient
	originalDelegateFactory := newDoltStartupDelegate
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		newDoltStartupDelegate = originalDelegateFactory
	})

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		if connectCalls == 1 {
			return &DoltClient{db: firstDB, details: details, beadsDir: beadsDir}, nil
		}
		return nil, &StartupError{Kind: StartupErrorKindServerStartup, Err: errors.New("connection refused"), Details: copyConnectionDetails(details)}
	}

	delegateCalled := false
	newDoltStartupDelegate = func(_ string) doltStartupDelegate {
		delegateCalled = true
		return &fakeDoltStartupDelegate{}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	attempted, reconnectErr := client.ReconnectIfRecoverable(errors.New("dial tcp 10.20.30.40:3306: connect: connection refused"))
	require.True(t, attempted)
	require.Error(t, reconnectErr)
	require.False(t, delegateCalled)
}

func TestDoltClientReconnect_NonRecoverableErrorDoesNotReconnect(t *testing.T) {
	client := &DoltClient{db: &sql.DB{}}

	attempted, err := client.ReconnectIfRecoverable(errors.New("Error 1064: You have an error in your SQL syntax"))
	require.False(t, attempted)
	require.NoError(t, err)
}

func TestDoltClientReconnect_DeduplicatesConcurrentReconnects(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	oldDB, _, err := sqlmock.New()
	require.NoError(t, err)
	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = newDB.Close() }()

	originalConnect := connectDoltClient
	t.Cleanup(func() { connectDoltClient = originalConnect })

	var mu sync.Mutex
	connectCalls := 0
	reconnectStarted := make(chan struct{})
	releaseReconnect := make(chan struct{})

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		mu.Lock()
		connectCalls++
		call := connectCalls
		mu.Unlock()

		switch call {
		case 1:
			return &DoltClient{db: oldDB, details: details, beadsDir: beadsDir}, nil
		case 2:
			close(reconnectStarted)
			<-releaseReconnect
			return &DoltClient{db: newDB, details: details, beadsDir: beadsDir}, nil
		default:
			return nil, fmt.Errorf("unexpected connect call: %d", call)
		}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	resultCh := make(chan struct {
		attempted bool
		err       error
	}, 1)
	go func() {
		attempted, reconnectErr := client.ReconnectIfRecoverable(errors.New("dial tcp 127.0.0.1:4010: connect: connection refused"))
		resultCh <- struct {
			attempted bool
			err       error
		}{attempted: attempted, err: reconnectErr}
	}()

	<-reconnectStarted
	waitCh, leader := client.beginReconnectAttempt()
	require.False(t, leader)
	require.NotNil(t, waitCh)
	close(releaseReconnect)
	result := <-resultCh
	require.True(t, result.attempted)
	require.NoError(t, result.err)
	<-waitCh
	require.NoError(t, client.lastReconnectResult())

	require.Equal(t, newDB, client.DB())
	require.Equal(t, 2, connectCalls)
	require.Error(t, oldDB.Ping())
}

func TestNewDoltClient_StartsHealthMonitorOnSuccessfulConnection(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	originalConnect := connectDoltClient
	originalCreateHealthMonitor := createHealthMonitor
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		createHealthMonitor = originalCreateHealthMonitor
	})

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return &DoltClient{db: db, details: details, beadsDir: beadsDir}, nil
	}

	called := 0
	var usedInterval time.Duration
	createHealthMonitor = func(client healthMonitorClient, ping healthPingFunc, interval time.Duration) clientHealthMonitor {
		called++
		require.NotNil(t, client)
		require.NotNil(t, ping)
		usedInterval = interval
		return &fakeClientHealthMonitor{}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, 1, called)
	require.Equal(t, defaultHealthMonitorInterval, usedInterval)
}

func TestDoltClientClose_StopsHealthMonitor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	mock.ExpectClose()

	healthMon := &fakeClientHealthMonitor{}
	client := &DoltClient{db: db, connectivityBroker: nil, healthMon: healthMon}

	require.NoError(t, client.Close())
	require.Equal(t, 1, healthMon.stopCalls)
	require.Error(t, db.Ping())
	require.NoError(t, mock.ExpectationsWereMet())

	require.NoError(t, client.Close())
	require.Equal(t, 1, healthMon.stopCalls)
}

func TestNewDoltClient_ConstructorOptionsRemainBackwardCompatible(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	originalConnect := connectDoltClient
	originalCreateHealthMonitor := createHealthMonitor
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		createHealthMonitor = originalCreateHealthMonitor
	})

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return &DoltClient{db: db, details: details, beadsDir: beadsDir}, nil
	}

	var intervals []time.Duration
	createHealthMonitor = func(_ healthMonitorClient, _ healthPingFunc, interval time.Duration) clientHealthMonitor {
		intervals = append(intervals, interval)
		return &fakeClientHealthMonitor{}
	}

	legacyClient, err := NewDoltClient(beadsDir)
	require.NoError(t, err)
	require.NotNil(t, legacyClient)

	optionClient, err := NewDoltClient(beadsDir, WithHealthCheckInterval(7*time.Second))
	require.NoError(t, err)
	require.NotNil(t, optionClient)

	require.Equal(t, []time.Duration{defaultHealthMonitorInterval, 7 * time.Second}, intervals)
}

func TestNewDoltMySQLConfig_UsesSafeDefaults(t *testing.T) {
	details := ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"}

	cfg := newDoltMySQLConfig(details)

	require.Equal(t, "root", cfg.User)
	require.Equal(t, "tcp", cfg.Net)
	require.Equal(t, "127.0.0.1:3306", cfg.Addr)
	require.Equal(t, "perles", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, doltDBDialTimeout, cfg.Timeout)
	require.Equal(t, doltDBReadTimeout, cfg.ReadTimeout)
	require.Equal(t, doltDBWriteTimeout, cfg.WriteTimeout)
	require.NotNil(t, cfg.Logger)
}

func TestConfigureDoltDBPool_SetsConnectionBounds(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	configureDoltDBPool(db)

	stats := db.Stats()
	require.Equal(t, doltDBMaxOpenConns, stats.MaxOpenConnections)

	// MaxIdleConns is not exposed directly in stats, but idle count should never exceed it.
	for range doltDBMaxIdleConns + 2 {
		require.NoError(t, db.Ping())
	}
	require.LessOrEqual(t, db.Stats().Idle, doltDBMaxIdleConns)
}

func TestDoltMySQLLogger_RoutesDriverMessagesToPerlesLog(t *testing.T) {
	cleanup, err := log.InitWithTeaLog(t.TempDir()+"/driver.log", "")
	require.NoError(t, err)
	defer cleanup()

	logger := doltMySQLLogger{}
	require.NotPanics(t, func() {
		logger.Print("connection.go:706 ", "closing bad idle connection: ", io.EOF)
	})
}

func TestNewDoltMySQLConfig_LoggerSuppressesDefaultStderrLeak(t *testing.T) {
	cfg := newDoltMySQLConfig(ConnectionDetails{Host: "localhost", Port: 3306, Database: "perles"})
	require.IsType(t, doltMySQLLogger{}, cfg.Logger)
	require.NotEqual(t, mysql.NopLogger{}, cfg.Logger)
}

func TestDoltClientReconnect_RecoversFromEOFAsRecoverableIdleFailure(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	oldDB, _, err := sqlmock.New()
	require.NoError(t, err)
	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = newDB.Close() }()

	originalConnect := connectDoltClient
	t.Cleanup(func() { connectDoltClient = originalConnect })

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		switch connectCalls {
		case 1:
			return &DoltClient{db: oldDB, details: details, beadsDir: beadsDir}, nil
		case 2:
			return &DoltClient{db: newDB, details: details, beadsDir: beadsDir}, nil
		default:
			return nil, fmt.Errorf("unexpected connect call: %d", connectCalls)
		}
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	attempted, reconnectErr := client.ReconnectIfRecoverable(io.EOF)
	require.True(t, attempted)
	require.NoError(t, reconnectErr)
	require.Equal(t, 2, connectCalls)
	require.Equal(t, newDB, client.DB())
}

func TestDoltClientReconnect_RecoversFromMySQLInvalidConnectionError(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	oldDB, _, err := sqlmock.New()
	require.NoError(t, err)
	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = newDB.Close() }()

	originalConnect := connectDoltClient
	t.Cleanup(func() { connectDoltClient = originalConnect })

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		if connectCalls == 1 {
			return &DoltClient{db: oldDB, details: details, beadsDir: beadsDir}, nil
		}
		return &DoltClient{db: newDB, details: details, beadsDir: beadsDir}, nil
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	attempted, reconnectErr := client.ReconnectIfRecoverable(errors.New("invalid connection"))
	require.True(t, attempted)
	require.NoError(t, reconnectErr)
	require.Equal(t, 2, connectCalls)
	require.Equal(t, newDB, client.DB())
}

func TestDoltClientReconnect_RecoversFromUnexpectedEOF(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	oldDB, _, err := sqlmock.New()
	require.NoError(t, err)
	newDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = newDB.Close() }()

	originalConnect := connectDoltClient
	t.Cleanup(func() { connectDoltClient = originalConnect })

	connectCalls := 0
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		connectCalls++
		if connectCalls == 1 {
			return &DoltClient{db: oldDB, details: details, beadsDir: beadsDir}, nil
		}
		return &DoltClient{db: newDB, details: details, beadsDir: beadsDir}, nil
	}

	client, err := NewDoltClient(beadsDir)
	require.NoError(t, err)

	attempted, reconnectErr := client.ReconnectIfRecoverable(io.ErrUnexpectedEOF)
	require.True(t, attempted)
	require.NoError(t, reconnectErr)
	require.Equal(t, 2, connectCalls)
	require.Equal(t, newDB, client.DB())
}

func TestDoltMySQLConfig_UsesJoinedTCPAddress(t *testing.T) {
	cfg := newDoltMySQLConfig(ConnectionDetails{Host: "::1", Port: 3306, Database: "perles"})
	require.Equal(t, net.JoinHostPort("::1", "3306"), cfg.Addr)
}

func TestDiagnosticContext_ReturnsClientSnapshot(t *testing.T) {
	now := time.Now().Add(-time.Minute)
	client := &DoltClient{
		details:                     ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"},
		connectivityState:           appbeads.ConnectivityStateDegraded,
		lastConnectivityStateChange: now,
		delegatedStartAttempted:     true,
		portSource:                  "dolt-server.port",
		lastConnectivityErr:         errors.New("project identity mismatch"),
	}

	d := client.DiagnosticContext()
	require.Equal(t, "127.0.0.1", d.Host)
	require.Equal(t, 3306, d.Port)
	require.Equal(t, "perles", d.Database)
	require.Equal(t, appbeads.ConnectivityStateDegraded, d.LastState)
	require.Equal(t, now, d.LastStateChange)
	require.True(t, d.DelegatedStartAttempted)
	require.Equal(t, "dolt-server.port", d.PortSource)
	require.Equal(t, "Run 'bd bootstrap' to repair project configuration and database wiring, then retry.", d.Suggestion)
}

func TestResolveConnectionDetailsWithSource_TracksPortSource(t *testing.T) {
	t.Run("config fallback", func(t *testing.T) {
		beadsDir := t.TempDir()
		writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
		writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3810)

		resolved, err := resolveConnectionDetailsWithSource(beadsDir)
		require.NoError(t, err)
		require.Equal(t, 3810, resolved.details.Port)
		require.Equal(t, "config.yaml", resolved.portSource)
	})

	t.Run("port override", func(t *testing.T) {
		beadsDir := t.TempDir()
		writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
		writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3810)
		writeTestPortFile(t, beadsDir, "3920")

		resolved, err := resolveConnectionDetailsWithSource(beadsDir)
		require.NoError(t, err)
		require.Equal(t, 3920, resolved.details.Port)
		require.Equal(t, "dolt-server.port", resolved.portSource)
	})
}

func TestDiagnosticSuggestionGeneration(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "server unreachable", err: errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"), want: "Ensure this project is using beads v1 server mode and run 'bd bootstrap'; if the server is stopped, run 'bd dolt start'."},
		{name: "project mismatch", err: errors.New("project identity mismatch for workspace"), want: "Run 'bd bootstrap' to repair project configuration and database wiring, then retry."},
		{name: "circuit breaker", err: errors.New("circuit breaker open"), want: "Run 'bd bootstrap' to repair runtime state; if needed, restart the Dolt server and retry."},
		{name: "port mismatch", err: errors.New("parsing dolt server port file \"x/dolt-server.port\": invalid syntax"), want: "Run 'bd bootstrap' to refresh server connection metadata, then retry Perles."},
		{name: "dolt panic", err: errors.New("panic: concurrent map writes"), want: "Restart the Dolt server for this project, then run 'bd bootstrap' and retry."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, suggestionForError(tt.err))
		})
	}
}

func TestDiagnosticStateSnapshotAndEventPublishing(t *testing.T) {
	client := &DoltClient{
		details:            ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"},
		portSource:         "config.yaml",
		connectivityBroker: pubsub.NewBroker[appbeads.ConnectivityEvent](),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := client.ConnectivityBroker().Subscribe(ctx)

	client.setConnectivityState(appbeads.ConnectivityStateReconnecting, errors.New("dial tcp 127.0.0.1:3306: connect: connection refused"))

	var event pubsub.Event[appbeads.ConnectivityEvent]
	select {
	case event = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for connectivity event")
	}

	require.Equal(t, appbeads.ConnectivityStateReconnecting, event.Payload.State)
	require.NotNil(t, event.Payload.Diagnostics)
	require.Equal(t, "127.0.0.1", event.Payload.Diagnostics.Host)
	require.Equal(t, 3306, event.Payload.Diagnostics.Port)
	require.Equal(t, "perles", event.Payload.Diagnostics.Database)
	require.Equal(t, appbeads.ConnectivityStateReconnecting, event.Payload.Diagnostics.LastState)
	require.False(t, event.Payload.Diagnostics.LastStateChange.IsZero())
	require.Equal(t, "config.yaml", event.Payload.Diagnostics.PortSource)
	require.Equal(t, "Ensure this project is using beads v1 server mode and run 'bd bootstrap'; if the server is stopped, run 'bd dolt start'.", event.Payload.Diagnostics.Suggestion)

	snapshot := client.DiagnosticContext()
	require.Equal(t, snapshot, *event.Payload.Diagnostics)
}

func TestStartupErrorsIncludeSuggestion(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "192.0.2.10", 3306)

	originalConnect := connectDoltClient
	defer func() { connectDoltClient = originalConnect }()

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     errors.New("dial tcp 192.0.2.10:3306: connect: connection refused"),
			Details: copyConnectionDetails(details),
		}
	}

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)

	var startupErr *StartupError
	require.True(t, errors.As(err, &startupErr))
	require.Equal(t, "Ensure this project is using beads v1 server mode and run 'bd bootstrap'; if the server is stopped, run 'bd dolt start'.", startupErr.Suggestion)
}

func TestValidateBeadsV1Compatibility_SucceedsForBeadsV1Schema(t *testing.T) {
	db := testutil.NewBeadsV1TestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	client := &DoltClient{db: db, details: ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"}}
	require.NoError(t, client.ValidateBeadsV1Compatibility())
}

func TestValidateBeadsV1Compatibility_FailsOnStaleSchema(t *testing.T) {
	db := testutil.NewTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	_, err := db.Exec(`ALTER TABLE issues DROP COLUMN hook_bead`)
	require.NoError(t, err)

	client := &DoltClient{db: db, details: ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"}}
	err = client.ValidateBeadsV1Compatibility()
	require.Error(t, err)
	require.True(t, IsCompatibilityError(err))
	require.Contains(t, err.Error(), "missing required tables/views")
	require.Contains(t, err.Error(), "config")
	require.Contains(t, err.Error(), "metadata")
	require.NotContains(t, err.Error(), "custom_statuses")
	require.NotContains(t, err.Error(), "custom_types")
	require.Contains(t, err.Error(), "missing required issues columns")
	require.Contains(t, err.Error(), "hook_bead")
}

func TestResolveConnectionDetails_SharedServerModeClassifiedAsNoBeads(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"shared-server","dolt_database":"perlesv1spec"}`)

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.True(t, IsNoBeadsError(err))
	require.Contains(t, err.Error(), `unsupported dolt mode "shared-server"`)
	require.Contains(t, StartupSuggestion(err), "supports beads v1+ only in dolt_mode=server")
}

func TestRetryDoltClientConnection_ReResolvesConnectionDetailsOnEachRetry(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)
	writeTestPortFile(t, beadsDir, "4010")

	originalConnect := connectDoltClient
	originalRetryPolicyFactory := startupRetryPolicyFactory
	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		startupRetryPolicyFactory = originalRetryPolicyFactory
		startupSleepWithContext = originalSleepWithContext
	})

	startupRetryPolicyFactory = func() RetryPolicy {
		return RetryPolicy{MaxAttempts: 5, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Jitter: 0}
	}
	startupSleepWithContext = func(context.Context, time.Duration) error { return nil }

	var seenPorts []int
	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		seenPorts = append(seenPorts, details.Port)
		switch len(seenPorts) {
		case 1:
			writeTestPortFile(t, beadsDir, "4020")
			return nil, &StartupError{Kind: StartupErrorKindServerStartup, Err: errors.New("first failure"), Details: copyConnectionDetails(details)}
		case 2:
			writeTestPortFile(t, beadsDir, "4030")
			return nil, &StartupError{Kind: StartupErrorKindServerStartup, Err: errors.New("second failure"), Details: copyConnectionDetails(details)}
		default:
			return &DoltClient{details: details}, nil
		}
	}

	client, err := retryDoltClientConnection(beadsDir)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, []int{4010, 4020, 4030}, seenPorts)
	require.Equal(t, 4030, client.details.Port)
}

func TestRetryDoltClientConnection_UsesStartupRetryPolicyBackoff(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 3306)

	originalConnect := connectDoltClient
	originalRetryPolicyFactory := startupRetryPolicyFactory
	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		connectDoltClient = originalConnect
		startupRetryPolicyFactory = originalRetryPolicyFactory
		startupSleepWithContext = originalSleepWithContext
	})

	policy := RetryPolicy{MaxAttempts: 4, InitialBackoff: 111 * time.Millisecond, MaxBackoff: 333 * time.Millisecond, Jitter: 0}
	startupRetryPolicyFactory = func() RetryPolicy { return policy }

	var slept []time.Duration
	startupSleepWithContext = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	connectDoltClient = func(_ string, details ConnectionDetails) (*DoltClient, error) {
		return nil, &StartupError{Kind: StartupErrorKindServerStartup, Err: fmt.Errorf("still unavailable on %d", details.Port), Details: copyConnectionDetails(details)}
	}

	_, err := retryDoltClientConnection(beadsDir)
	require.Error(t, err)
	require.Equal(t, []time.Duration{111 * time.Millisecond, 222 * time.Millisecond, 333 * time.Millisecond}, slept)
}

func TestPollForDoltPortFile_PollsEvery50msUpTo500ms(t *testing.T) {
	beadsDir := t.TempDir()

	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		startupSleepWithContext = originalSleepWithContext
	})

	var slept []time.Duration
	startupSleepWithContext = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}

	pollForDoltPortFile(context.Background(), beadsDir)

	require.Len(t, slept, 10)
	for _, d := range slept {
		require.Equal(t, 50*time.Millisecond, d)
	}
}

func TestPollForDoltPortFile_StopsWhenFileAppears(t *testing.T) {
	beadsDir := t.TempDir()

	originalSleepWithContext := startupSleepWithContext
	t.Cleanup(func() {
		startupSleepWithContext = originalSleepWithContext
	})

	var slept []time.Duration
	startupSleepWithContext = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		if len(slept) == 3 {
			writeTestPortFile(t, beadsDir, "4020")
		}
		return nil
	}

	pollForDoltPortFile(context.Background(), beadsDir)

	require.Equal(t, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond, 50 * time.Millisecond}, slept)
}

func TestGetCommentsFailureAfterReconnectLogsFinalRootCause(t *testing.T) {
	logPath := setupTestLogger(t)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	issueID := "perles-123"
	queryErr := errors.New("dial tcp 127.0.0.1:3306: connect: connection refused")
	mock.ExpectQuery("FROM comments").WithArgs(issueID).WillReturnError(queryErr)

	client := &DoltClient{
		db:       db,
		beadsDir: filepath.Join(t.TempDir(), "missing-beads"),
		details:  ConnectionDetails{Host: "127.0.0.1", Port: 3306, Database: "perles"},
	}

	_, err = client.GetComments(issueID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading beads metadata")
	require.NoError(t, mock.ExpectationsWereMet())

	content := readLogFile(t, logPath)
	require.Contains(t, content, "[ERROR] [db] Reconnect exhausted; connectivity degraded")
	require.Contains(t, content, "operation=reconnect")
	require.Contains(t, content, "state_transition=degraded")
	require.Contains(t, content, "host=127.0.0.1")
	require.Contains(t, content, "port=3306")
	require.Contains(t, content, "database=perles")

	require.Contains(t, content, "[ERROR] [db] GetComments failed")
	require.Contains(t, content, "operation=get_comments")
	require.Contains(t, content, "issue_id=perles-123")
	require.Contains(t, content, "reconnect_attempted=true")
	require.Contains(t, content, "connectivity_state=degraded")
	require.Contains(t, content, "error=startup no_beads: reading beads metadata")
}

func TestLogStartupFailureIncludesActionableMissingDatabaseContext(t *testing.T) {
	logPath := setupTestLogger(t)

	startupErr := &StartupError{
		Kind: StartupErrorKindServerStartup,
		Err:  &mysql.MySQLError{Number: 1049, Message: "Unknown database 'perles'"},
		Details: &ConnectionDetails{
			Host:     "127.0.0.1",
			Port:     3306,
			Database: "perles",
		},
	}

	logStartupFailure("open_dolt_db_ping", startupErr, ConnectionDetails{})

	content := readLogFile(t, logPath)
	require.Contains(t, content, "[ERROR] [db] Dolt startup failed")
	require.Contains(t, content, "operation=open_dolt_db_ping")
	require.Contains(t, content, "host=127.0.0.1")
	require.Contains(t, content, "port=3306")
	require.Contains(t, content, "database=perles")
	require.Contains(t, content, "mysql_error_code=1049")
	require.Contains(t, content, "missing_database=true")
	require.Contains(t, content, "suggestion=Run 'bd bootstrap' to repair project configuration and database wiring, then retry.")
	require.Contains(t, content, "error=startup server_startup: Error 1049")
}

func setupTestLogger(t *testing.T) string {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "perles.log")
	cleanup, err := log.InitWithTeaLog(logPath, "")
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return logPath
}

func readLogFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(content)
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

type fakeClientHealthMonitor struct {
	stopCalls int
}

func (f *fakeClientHealthMonitor) Stop() {
	f.stopCalls++
}
