package infrastructure

import (
	"os"
	"path/filepath"
	"strconv"
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
	require.Contains(t, err.Error(), "unsupported beads backend")
}

func TestResolveConnectionDetails_InvalidPortFile(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 13849)
	writeTestPortFile(t, beadsDir, "nope")

	_, err := ResolveConnectionDetails(beadsDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parsing dolt server port")
}

func TestDoltClientFailsWhenServerUnavailable(t *testing.T) {
	beadsDir := t.TempDir()
	writeTestMetadata(t, beadsDir, `{"backend":"dolt","dolt_mode":"server","dolt_database":"perles"}`)
	writeTestDoltConfig(t, beadsDir, "127.0.0.1", 1)

	_, err := NewDoltClient(beadsDir)
	require.Error(t, err)
	require.Contains(t, err.Error(), "pinging dolt mysql connection")
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
