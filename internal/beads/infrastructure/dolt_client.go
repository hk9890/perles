package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	appbeads "github.com/hk9890/perles/internal/beads/application"
	domain "github.com/hk9890/perles/internal/beads/domain"
	"github.com/hk9890/perles/internal/log"
	"github.com/hk9890/perles/internal/pubsub"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

var _ appbeads.ReadClient = (*DoltClient)(nil)
var _ appbeads.Reconnector = (*DoltClient)(nil)
var _ appbeads.ConnectivityObserver = (*DoltClient)(nil)

type DoltClient struct {
	mu          sync.RWMutex
	reconnectMu sync.Mutex
	db          *sql.DB
	details     ConnectionDetails
	beadsDir    string

	connectivityState  appbeads.ConnectivityState
	connectivityBroker *pubsub.Broker[appbeads.ConnectivityEvent]
	reconnectWait      chan struct{}
	reconnectResult    error
}

type ConnectionDetails struct {
	Host     string
	Port     int
	Database string
}

type StartupErrorKind string

const (
	// StartupErrorKindNoBeads indicates this repository is not usable as a beads
	// Dolt server project (missing/invalid metadata or config).
	StartupErrorKindNoBeads StartupErrorKind = "no_beads"
	// StartupErrorKindServerStartup indicates server startup/connection failures
	// (for example dial/open/ping failures, invalid port state, delegated startup failures).
	StartupErrorKindServerStartup StartupErrorKind = "server_startup"
)

type StartupError struct {
	Kind    StartupErrorKind
	Err     error
	Details *ConnectionDetails
}

func (e *StartupError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("startup %s: %v", e.Kind, e.Err)
}

func (e *StartupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsNoBeadsError(err error) bool {
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		return false
	}
	return startupErr.Kind == StartupErrorKindNoBeads
}

func IsServerStartupError(err error) bool {
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		return false
	}
	return startupErr.Kind == StartupErrorKindServerStartup
}

type beadsMetadata struct {
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	DoltDatabase string `json:"dolt_database"`
}

type doltServerConfig struct {
	Listener struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"listener"`
}

type doltStartupDelegate interface {
	Start(ctx context.Context) error
}

var (
	connectDoltClient = openDoltClientWithDetails

	newDoltStartupDelegate = func(beadsDir string) doltStartupDelegate {
		return newBDDoltStarter(beadsDir)
	}

	postStartReadinessBackoff = []time.Duration{
		150 * time.Millisecond,
		300 * time.Millisecond,
		600 * time.Millisecond,
	}
)

func NewDoltClient(beadsDir string) (*DoltClient, error) {
	details, err := ResolveConnectionDetails(beadsDir)
	if err != nil {
		return nil, err
	}

	return connectOrDelegateStartup(beadsDir, details)
}

func connectOrDelegateStartup(beadsDir string, details ConnectionDetails) (*DoltClient, error) {
	client, err := connectDoltClient(beadsDir, details)
	if err == nil {
		return client, nil
	}

	if !shouldAttemptDelegatedStartup(details.Host, err) {
		return nil, err
	}

	if startErr := newDoltStartupDelegate(beadsDir).Start(context.Background()); startErr != nil {
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     startErr,
			Details: copyConnectionDetails(details),
		}
	}

	updatedDetails, err := ResolveConnectionDetails(beadsDir)
	if err != nil {
		return nil, err
	}

	return retryDoltClientConnection(beadsDir, updatedDetails)
}

func retryDoltClientConnection(beadsDir string, details ConnectionDetails) (*DoltClient, error) {
	var lastErr error

	for attempt := 0; attempt <= len(postStartReadinessBackoff); attempt++ {
		client, err := connectDoltClient(beadsDir, details)
		if err == nil {
			return client, nil
		}

		lastErr = err
		if !IsServerStartupError(err) {
			return nil, err
		}

		if attempt < len(postStartReadinessBackoff) {
			time.Sleep(postStartReadinessBackoff[attempt])
		}
	}

	return nil, lastErr
}

func shouldAttemptDelegatedStartup(host string, err error) bool {
	return IsServerStartupError(err) && isLocalDoltHost(host)
}

func isLocalDoltHost(host string) bool {
	switch strings.TrimSpace(strings.ToLower(host)) {
	case "", "localhost", "127.0.0.1", "::1", "[::1]", "0.0.0.0":
		return true
	default:
		return false
	}
}

func openDoltClientWithDetails(beadsDir string, details ConnectionDetails) (*DoltClient, error) {
	db, err := openDoltDB(details)
	if err != nil {
		return nil, err
	}

	return &DoltClient{
		db:                 db,
		details:            details,
		beadsDir:           beadsDir,
		connectivityState:  appbeads.ConnectivityStateHealthy,
		connectivityBroker: pubsub.NewBroker[appbeads.ConnectivityEvent](),
	}, nil
}

func openDoltDB(details ConnectionDetails) (*sql.DB, error) {

	dsn := fmt.Sprintf("root@tcp(%s:%d)/%s?parseTime=true", details.Host, details.Port, details.Database)
	log.Debug(log.CatDB, "Opening Dolt database", "host", details.Host, "port", details.Port, "database", details.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     fmt.Errorf("opening dolt mysql connection: %w", err),
			Details: copyConnectionDetails(details),
		}
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     fmt.Errorf("pinging dolt mysql connection: %w", err),
			Details: copyConnectionDetails(details),
		}
	}

	return db, nil
}

func (c *DoltClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	if c.connectivityBroker != nil {
		c.connectivityBroker.Close()
		c.connectivityBroker = nil
	}
	return err
}

func (c *DoltClient) DB() *sql.DB {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

func (c *DoltClient) ConnectivityState() appbeads.ConnectivityState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.connectivityState == "" {
		return appbeads.ConnectivityStateHealthy
	}
	return c.connectivityState
}

func (c *DoltClient) ConnectivityBroker() *pubsub.Broker[appbeads.ConnectivityEvent] {
	c.mu.Lock()
	if c.connectivityBroker == nil {
		c.connectivityBroker = pubsub.NewBroker[appbeads.ConnectivityEvent]()
	}
	if c.connectivityState == "" {
		c.connectivityState = appbeads.ConnectivityStateHealthy
	}
	broker := c.connectivityBroker
	c.mu.Unlock()
	return broker
}

func (c *DoltClient) Version() (string, error) {
	readVersion := func() (string, error) {
		db := c.DB()
		if db == nil {
			return "", errors.New("database connection unavailable")
		}

		var version string
		err := db.QueryRow("SELECT value FROM metadata WHERE `key` = ?", "bd_version").Scan(&version)
		if err != nil {
			return "", err
		}
		return version, nil
	}

	version, err := readVersion()
	if attempted, reconnectErr := c.ReconnectIfRecoverable(err); attempted {
		if reconnectErr != nil {
			return "", fmt.Errorf("reading bd_version from metadata: %w", reconnectErr)
		}
		version, err = readVersion()
	}
	if err != nil {
		return "", fmt.Errorf("reading bd_version from metadata: %w", err)
	}
	return version, nil
}

func (c *DoltClient) GetComments(issueID string) ([]domain.Comment, error) {
	query := `
		SELECT id, author, text, created_at
		FROM comments
		WHERE issue_id = ?
		ORDER BY created_at ASC
	`

	readComments := func() ([]domain.Comment, error) {
		db := c.DB()
		if db == nil {
			return nil, errors.New("database connection unavailable")
		}

		rows, err := db.Query(query, issueID)
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		var comments []domain.Comment
		for rows.Next() {
			var comment domain.Comment
			if err := rows.Scan(&comment.ID, &comment.Author, &comment.Text, &comment.CreatedAt); err != nil {
				return nil, err
			}
			comments = append(comments, comment)
		}

		return comments, rows.Err()
	}

	comments, err := readComments()
	if attempted, reconnectErr := c.ReconnectIfRecoverable(err); attempted {
		if reconnectErr != nil {
			return nil, reconnectErr
		}
		return readComments()
	}

	return comments, err
}

func (c *DoltClient) ReconnectIfRecoverable(err error) (bool, error) {
	if !appbeads.IsRecoverableConnectivityError(err) {
		return false, nil
	}

	waitCh, leader := c.beginReconnectAttempt()
	if !leader {
		<-waitCh
		return true, c.lastReconnectResult()
	}

	c.setConnectivityState(appbeads.ConnectivityStateReconnecting, nil)

	reconnectErr := c.reconnect()
	c.finishReconnectAttempt(reconnectErr)

	return true, reconnectErr

}

func (c *DoltClient) beginReconnectAttempt() (<-chan struct{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.connectivityState == "" {
		c.connectivityState = appbeads.ConnectivityStateHealthy
	}
	if c.reconnectWait != nil {
		return c.reconnectWait, false
	}
	c.reconnectWait = make(chan struct{})
	c.reconnectResult = nil
	return c.reconnectWait, true
}

func (c *DoltClient) finishReconnectAttempt(reconnectErr error) {
	state := appbeads.ConnectivityStateHealthy
	if reconnectErr != nil {
		state = appbeads.ConnectivityStateDegraded
	}

	c.mu.Lock()
	waitCh := c.reconnectWait
	c.reconnectResult = reconnectErr
	c.mu.Unlock()

	c.setConnectivityState(state, reconnectErr)

	c.mu.Lock()
	if c.reconnectWait == waitCh {
		c.reconnectWait = nil
	}
	c.mu.Unlock()

	if waitCh != nil {
		close(waitCh)
	}
}

func (c *DoltClient) lastReconnectResult() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnectResult

}

func (c *DoltClient) setConnectivityState(state appbeads.ConnectivityState, err error) {
	c.mu.Lock()
	if c.connectivityState == "" {
		c.connectivityState = appbeads.ConnectivityStateHealthy
	}
	if c.connectivityBroker == nil {
		c.connectivityBroker = pubsub.NewBroker[appbeads.ConnectivityEvent]()
	}
	if c.connectivityState == state {
		c.mu.Unlock()
		return
	}
	c.connectivityState = state
	broker := c.connectivityBroker
	c.mu.Unlock()

	broker.Publish(pubsub.UpdatedEvent, appbeads.ConnectivityEvent{State: state, Err: err})
}

func (c *DoltClient) reconnect() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	details, err := ResolveConnectionDetails(c.beadsDir)
	if err != nil {
		return err
	}

	newClient, err := connectOrDelegateStartup(c.beadsDir, details)
	if err != nil {
		return err
	}

	newDB := newClient.DB()

	c.mu.Lock()
	oldDB := c.db
	c.db = newDB
	c.details = details
	c.mu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}

	return nil
}

func ResolveConnectionDetails(beadsDir string) (ConnectionDetails, error) {
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath) //nolint:gosec // metadataPath is a fixed file under the resolved .beads directory
	if err != nil {
		// no_beads: repository is missing required beads metadata.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("reading beads metadata %q: %w", metadataPath, err),
		}
	}

	var metadata beadsMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		// no_beads: beads metadata exists but is invalid/unreadable.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("parsing beads metadata %q: %w", metadataPath, err),
		}
	}

	if metadata.Backend != "dolt" {
		// no_beads: backend is not supported by this Dolt client.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("unsupported beads backend %q; expected dolt", metadata.Backend),
		}
	}
	if metadata.DoltMode != "server" {
		// no_beads: project is not configured for Dolt server mode.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("unsupported dolt mode %q; expected server", metadata.DoltMode),
		}
	}
	if strings.TrimSpace(metadata.DoltDatabase) == "" {
		// no_beads: metadata is missing required connection fields.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  errors.New("missing dolt_database in beads metadata"),
		}
	}

	host, cfgPort, err := readDoltConfig(filepath.Join(beadsDir, "dolt", "config.yaml"))
	if err != nil {
		// no_beads: connection config is missing/invalid.
		return ConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  err,
		}
	}

	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}

	configDetails := ConnectionDetails{
		Host:     host,
		Port:     cfgPort,
		Database: metadata.DoltDatabase,
	}

	port, err := readDoltPort(filepath.Join(beadsDir, "dolt-server.port"), cfgPort)
	if err != nil {
		// server_startup: port file parse/validation/read failures indicate
		// startup/runtime state, not project identity.
		return ConnectionDetails{}, &StartupError{
			Kind:    StartupErrorKindServerStartup,
			Err:     err,
			Details: copyConnectionDetails(configDetails),
		}
	}

	return ConnectionDetails{
		Host:     configDetails.Host,
		Port:     port,
		Database: configDetails.Database,
	}, nil
}

func copyConnectionDetails(details ConnectionDetails) *ConnectionDetails {
	copy := details
	return &copy
}

func readDoltConfig(path string) (host string, port int, err error) {
	content, err := os.ReadFile(path) //nolint:gosec // path points to the tracked Dolt config under the resolved .beads directory
	if err != nil {
		return "", 0, fmt.Errorf("reading dolt config %q: %w", path, err)
	}

	var cfg doltServerConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return "", 0, fmt.Errorf("parsing dolt config %q: %w", path, err)
	}

	if cfg.Listener.Port == 0 {
		return "", 0, fmt.Errorf("missing listener.port in dolt config %q", path)
	}

	return cfg.Listener.Host, cfg.Listener.Port, nil
}

func readDoltPort(path string, fallback int) (int, error) {
	content, err := os.ReadFile(path) //nolint:gosec // path points to the tracked Dolt port file under the resolved .beads directory
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return 0, fmt.Errorf("reading dolt server port file %q: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return fallback, nil
	}

	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, fmt.Errorf("parsing dolt server port file %q: %w", path, err)
	}

	if port <= 0 {
		return 0, fmt.Errorf("invalid dolt server port %d in %q", port, path)
	}

	return port, nil
}
