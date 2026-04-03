package infrastructure

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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

	mysql "github.com/go-sql-driver/mysql"
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
	healthMon   clientHealthMonitor

	connectivityState           appbeads.ConnectivityState
	lastConnectivityStateChange time.Time
	lastConnectivityErr         error
	delegatedStartAttempted     bool
	portSource                  string
	connectivityBroker          *pubsub.Broker[appbeads.ConnectivityEvent]
	reconnectWait               chan struct{}
	reconnectResult             error
}

type clientHealthMonitor interface {
	Stop()
}

type DoltClientOption func(*doltClientOptions)

type doltClientOptions struct {
	healthCheckInterval time.Duration
}

func defaultDoltClientOptions() doltClientOptions {
	return doltClientOptions{healthCheckInterval: defaultHealthMonitorInterval}
}

func WithHealthCheckInterval(d time.Duration) DoltClientOption {
	return func(opts *doltClientOptions) {
		opts.healthCheckInterval = d
	}
}

type ConnectionDetails struct {
	Host     string
	Port     int
	Database string
}

type resolvedConnectionDetails struct {
	details    ConnectionDetails
	portSource string
}

type StartupErrorKind string

const (
	// StartupErrorKindNoBeads indicates this repository is not usable as a beads
	// Dolt server project (missing/invalid metadata or config).
	StartupErrorKindNoBeads StartupErrorKind = "no_beads"
	// StartupErrorKindServerStartup indicates server startup/connection failures
	// (for example dial/open/ping failures, invalid port state, delegated startup failures).
	StartupErrorKindServerStartup StartupErrorKind = "server_startup"
	// StartupErrorKindCompatibility indicates a beads project that is discoverable
	// but incompatible with Perles' supported beads runtime/schema contract.
	StartupErrorKindCompatibility StartupErrorKind = "compatibility"
)

type StartupError struct {
	Kind       StartupErrorKind
	Err        error
	Details    *ConnectionDetails
	Suggestion string
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

func IsCompatibilityError(err error) bool {
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		return false
	}
	return startupErr.Kind == StartupErrorKindCompatibility
}

func StartupSuggestion(err error) string {
	if err == nil {
		return ""
	}

	var startupErr *StartupError
	if errors.As(err, &startupErr) && startupErr.Suggestion != "" {
		return startupErr.Suggestion
	}

	return suggestionForError(err)
}

type beadsMetadata struct {
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	DoltDatabase string `json:"dolt_database"`
}

var requiredBeadsV1Relations = []string{
	"issues",
	"dependencies",
	"labels",
	"comments",
	"config",
	"metadata",
	"custom_statuses",
	"custom_types",
	"blocked_issues",
	"ready_issues",
}

var requiredBeadsV1IssueColumns = []string{
	"id",
	"title",
	"description",
	"design",
	"acceptance_criteria",
	"notes",
	"status",
	"priority",
	"issue_type",
	"assignee",
	"created_at",
	"created_by",
	"updated_at",
	"closed_at",
	"close_reason",
	"ephemeral",
	"pinned",
	"is_template",
	"defer_until",
	"due_at",
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
	connectDoltClient   = openDoltClientWithDetails
	createHealthMonitor = func(client healthMonitorClient, ping healthPingFunc, interval time.Duration) clientHealthMonitor {
		return NewHealthMonitor(client, ping, interval)
	}

	newDoltStartupDelegate = func(beadsDir string) doltStartupDelegate {
		return newBDDoltStarter(beadsDir)
	}

	startupRetryPolicyFactory = DefaultStartupRetryPolicy
	startupSleepWithContext   = sleepWithContext
)

const (
	startupPortFilePollInterval = 50 * time.Millisecond
	startupPortFilePollTimeout  = 500 * time.Millisecond
)

func init() {
	if err := mysql.SetLogger(doltMySQLLogger{}); err != nil {
		log.Warn(log.CatDB, "Failed to install MySQL driver logger", "error", err)
	}
}

const (
	doltDBDialTimeout     = 5 * time.Second
	doltDBReadTimeout     = 5 * time.Second
	doltDBWriteTimeout    = 5 * time.Second
	doltDBConnMaxIdleTime = 30 * time.Second
	doltDBConnMaxLifetime = 5 * time.Minute
	doltDBMaxIdleConns    = 1
	doltDBMaxOpenConns    = 4
)

func NewDoltClient(beadsDir string, opts ...DoltClientOption) (*DoltClient, error) {
	options := defaultDoltClientOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}

	resolved, err := resolveConnectionDetailsWithSource(beadsDir)
	if err != nil {
		logStartupFailure("resolve_connection_details", err, ConnectionDetails{})
		return nil, err
	}

	client, _, err := connectOrDelegateStartup(beadsDir, resolved)
	if err != nil {
		logStartupFailure("connect_or_delegate_startup", err, resolved.details)
		return nil, err
	}

	client.startHealthMonitor(options.healthCheckInterval)
	return client, nil
}

func connectOrDelegateStartup(beadsDir string, resolved resolvedConnectionDetails) (*DoltClient, bool, error) {
	client, err := connectDoltClient(beadsDir, resolved.details)
	if err == nil {
		client.mu.Lock()
		client.portSource = resolved.portSource
		client.mu.Unlock()
		return client, false, nil
	}
	err = enrichStartupError(err, resolved.details)

	if !shouldAttemptDelegatedStartup(resolved.details.Host, err) {
		return nil, false, err
	}

	if startErr := newDoltStartupDelegate(beadsDir).Start(context.Background()); startErr != nil {
		return nil, true, &StartupError{
			Kind:       StartupErrorKindServerStartup,
			Err:        startErr,
			Details:    copyConnectionDetails(resolved.details),
			Suggestion: suggestionForError(startErr),
		}
	}

	pollForDoltPortFile(context.Background(), beadsDir)

	client, retryErr := retryDoltClientConnection(beadsDir)
	if retryErr != nil {
		return nil, true, retryErr
	}

	client.mu.Lock()
	client.delegatedStartAttempted = true
	client.mu.Unlock()

	return client, true, nil
}

func retryDoltClientConnection(beadsDir string) (*DoltClient, error) {
	policy := startupRetryPolicyFactory()
	maxAttempts := policy.maxAttempts()
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		resolved, err := resolveConnectionDetailsWithSource(beadsDir)
		if err != nil {
			err = enrichStartupError(err, ConnectionDetails{})
			lastErr = err
			if !IsServerStartupError(err) || attempt == maxAttempts {
				logStartupFailure("retry_startup_resolve_connection_details", err, ConnectionDetails{})
				return nil, err
			}

			if sleepErr := startupSleepWithContext(context.Background(), policy.backoffForAttempt(attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}

		log.Debug(log.CatDB, "Retrying Dolt startup connection", "attempt", attempt, "max_attempts", maxAttempts, "host", resolved.details.Host, "port", resolved.details.Port)

		client, err := connectDoltClient(beadsDir, resolved.details)
		if err == nil {
			client.mu.Lock()
			client.portSource = resolved.portSource
			client.mu.Unlock()
			return client, nil
		}

		err = enrichStartupError(err, resolved.details)
		lastErr = err
		if !IsServerStartupError(err) {
			logStartupFailure("retry_startup_connect", err, resolved.details)
			return nil, err
		}

		if attempt < maxAttempts {
			if sleepErr := startupSleepWithContext(context.Background(), policy.backoffForAttempt(attempt)); sleepErr != nil {
				return nil, sleepErr
			}
		}
	}

	if lastErr != nil {
		logStartupFailure("retry_startup_exhausted", lastErr, ConnectionDetails{})
	}
	return nil, lastErr
}

func pollForDoltPortFile(ctx context.Context, beadsDir string) {
	portFilePath := filepath.Join(beadsDir, "dolt-server.port")
	if _, err := os.Stat(portFilePath); err == nil {
		log.Debug(log.CatDB, "Dolt port file already present before startup retries", "path", portFilePath)
		return
	}

	log.Debug(log.CatDB, "Polling for Dolt port file before startup retries", "path", portFilePath, "interval", startupPortFilePollInterval, "timeout", startupPortFilePollTimeout)
	for elapsed := time.Duration(0); elapsed < startupPortFilePollTimeout; elapsed += startupPortFilePollInterval {
		if err := startupSleepWithContext(ctx, startupPortFilePollInterval); err != nil {
			log.Debug(log.CatDB, "Stopped Dolt port file polling before startup retries", "path", portFilePath, "error", err)
			return
		}

		if _, err := os.Stat(portFilePath); err == nil {
			log.Debug(log.CatDB, "Dolt port file detected before startup retries", "path", portFilePath)
			return
		}
	}

	log.Debug(log.CatDB, "Dolt port file not detected before startup retries; continuing with config fallback", "path", portFilePath)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		if ctx != nil {
			return ctx.Err()
		}
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
		db:                          db,
		details:                     details,
		beadsDir:                    beadsDir,
		connectivityState:           appbeads.ConnectivityStateHealthy,
		lastConnectivityStateChange: time.Now(),
		connectivityBroker:          pubsub.NewBroker[appbeads.ConnectivityEvent](),
	}, nil
}

type doltMySQLLogger struct{}

func (doltMySQLLogger) Print(v ...any) {
	msg := strings.TrimSpace(fmt.Sprintln(v...))
	if msg == "" {
		return
	}
	log.Debug(log.CatDB, "mysql driver", "message", msg)
}

func newDoltMySQLConfig(details ConnectionDetails) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(details.Host, strconv.Itoa(details.Port))
	cfg.DBName = details.Database
	cfg.ParseTime = true
	cfg.Timeout = doltDBDialTimeout
	cfg.ReadTimeout = doltDBReadTimeout
	cfg.WriteTimeout = doltDBWriteTimeout
	cfg.Logger = doltMySQLLogger{}
	return cfg
}

func configureDoltDBPool(db *sql.DB) {
	if db == nil {
		return
	}
	db.SetMaxIdleConns(doltDBMaxIdleConns)
	db.SetMaxOpenConns(doltDBMaxOpenConns)
	db.SetConnMaxIdleTime(doltDBConnMaxIdleTime)
	db.SetConnMaxLifetime(doltDBConnMaxLifetime)
}

func openDoltDB(details ConnectionDetails) (*sql.DB, error) {
	cfg := newDoltMySQLConfig(details)
	log.Debug(log.CatDB, "Opening Dolt database", "host", details.Host, "port", details.Port, "database", details.Database)

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		startupErr := &StartupError{
			Kind:       StartupErrorKindServerStartup,
			Err:        fmt.Errorf("creating dolt mysql connector: %w", err),
			Details:    copyConnectionDetails(details),
			Suggestion: suggestionForError(err),
		}
		logStartupFailure("open_dolt_db_connector", startupErr, details)
		return nil, startupErr
	}
	db := sql.OpenDB(connector)
	configureDoltDBPool(db)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		startupErr := &StartupError{
			Kind:       StartupErrorKindServerStartup,
			Err:        fmt.Errorf("pinging dolt mysql connection: %w", err),
			Details:    copyConnectionDetails(details),
			Suggestion: suggestionForError(err),
		}
		logStartupFailure("open_dolt_db_ping", startupErr, details)
		return nil, startupErr
	}

	return db, nil
}

func (c *DoltClient) Close() error {
	c.mu.Lock()
	healthMon := c.healthMon
	c.healthMon = nil
	db := c.db
	c.db = nil
	broker := c.connectivityBroker
	c.connectivityBroker = nil
	c.mu.Unlock()

	if healthMon != nil {
		healthMon.Stop()
	}

	var err error
	if db != nil {
		err = db.Close()
	}
	if broker != nil {
		broker.Close()
	}

	return err
}

func (c *DoltClient) startHealthMonitor(interval time.Duration) {
	c.mu.Lock()
	if c.healthMon != nil {
		existing := c.healthMon
		c.healthMon = nil
		c.mu.Unlock()
		existing.Stop()
		c.mu.Lock()
	}
	c.healthMon = createHealthMonitor(c, c.pingContext, interval)
	c.mu.Unlock()
}

func (c *DoltClient) pingContext(ctx context.Context) error {
	db := c.DB()
	if db == nil {
		return errors.New("database connection unavailable")
	}
	return db.PingContext(ctx)
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
	if c.lastConnectivityStateChange.IsZero() {
		c.lastConnectivityStateChange = time.Now()
	}
	broker := c.connectivityBroker
	c.mu.Unlock()
	return broker
}

func (c *DoltClient) DiagnosticContext() appbeads.DiagnosticContext {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.connectivityState
	if state == "" {
		state = appbeads.ConnectivityStateHealthy
	}

	return appbeads.DiagnosticContext{
		Host:                    c.details.Host,
		Port:                    c.details.Port,
		Database:                c.details.Database,
		LastState:               state,
		LastStateChange:         c.lastConnectivityStateChange,
		DelegatedStartAttempted: c.delegatedStartAttempted,
		PortSource:              c.portSource,
		Suggestion:              suggestionForError(c.lastConnectivityErr),
	}
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

func (c *DoltClient) ValidateBeadsV1Compatibility() error {
	db := c.DB()
	if db == nil {
		return &StartupError{
			Kind:       StartupErrorKindCompatibility,
			Err:        errors.New("database connection unavailable"),
			Details:    copyConnectionDetails(c.details),
			Suggestion: "Run 'bd bootstrap' to repair your beads v1 project, then retry Perles.",
		}
	}

	missingRelations := make([]string, 0)
	for _, relation := range requiredBeadsV1Relations {
		query := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", relation)
		if _, err := db.Exec(query); err != nil {
			if appbeads.IsMissingTableError(err) {
				missingRelations = append(missingRelations, relation)
				continue
			}

			return &StartupError{
				Kind:       StartupErrorKindCompatibility,
				Err:        fmt.Errorf("checking required relation %q: %w", relation, err),
				Details:    copyConnectionDetails(c.details),
				Suggestion: "Run 'bd bootstrap' to repair your beads v1 project, then retry Perles.",
			}
		}
	}

	rows, err := db.Query("SELECT * FROM issues LIMIT 0")
	if err != nil {
		if appbeads.IsMissingTableError(err) {
			missingRelations = append(missingRelations, "issues")
		} else {
			return &StartupError{
				Kind:       StartupErrorKindCompatibility,
				Err:        fmt.Errorf("checking required issues columns: %w", err),
				Details:    copyConnectionDetails(c.details),
				Suggestion: "Run 'bd bootstrap' to repair your beads v1 project, then retry Perles.",
			}
		}
	}

	missingColumns := make([]string, 0)
	if rows != nil {
		defer func() { _ = rows.Close() }()

		available, colErr := rows.Columns()
		if colErr != nil {
			return &StartupError{
				Kind:       StartupErrorKindCompatibility,
				Err:        fmt.Errorf("reading issues column metadata: %w", colErr),
				Details:    copyConnectionDetails(c.details),
				Suggestion: "Run 'bd bootstrap' to repair your beads v1 project, then retry Perles.",
			}
		}

		availableSet := make(map[string]struct{}, len(available))
		for _, col := range available {
			availableSet[strings.ToLower(col)] = struct{}{}
		}

		for _, requiredCol := range requiredBeadsV1IssueColumns {
			if _, ok := availableSet[strings.ToLower(requiredCol)]; !ok {
				missingColumns = append(missingColumns, requiredCol)
			}
		}
	}

	if len(missingRelations) == 0 && len(missingColumns) == 0 {
		return nil
	}

	reasons := make([]string, 0, 2)
	if len(missingRelations) > 0 {
		reasons = append(reasons, fmt.Sprintf("missing required tables/views: %s", strings.Join(missingRelations, ", ")))
	}
	if len(missingColumns) > 0 {
		reasons = append(reasons, fmt.Sprintf("missing required issues columns: %s", strings.Join(missingColumns, ", ")))
	}

	return &StartupError{
		Kind:       StartupErrorKindCompatibility,
		Err:        errors.New(strings.Join(reasons, "; ")),
		Details:    copyConnectionDetails(c.details),
		Suggestion: "Run 'bd bootstrap' to apply/repair the beads v1 schema, then retry Perles.",
	}
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
			var rawCommentID any
			var comment domain.Comment
			if err := rows.Scan(&rawCommentID, &comment.Author, &comment.Text, &comment.CreatedAt); err != nil {
				return nil, err
			}

			comment.ID, err = normalizeCommentID(rawCommentID)
			if err != nil {
				return nil, fmt.Errorf("normalize comment id: %w", err)
			}
			comments = append(comments, comment)
		}

		return comments, rows.Err()
	}

	comments, err := readComments()
	if attempted, reconnectErr := c.ReconnectIfRecoverable(err); attempted {
		if reconnectErr != nil {
			c.logGetCommentsFailure(issueID, reconnectErr, true)
			return nil, reconnectErr
		}
		comments, err = readComments()
		if err != nil {
			c.logGetCommentsFailure(issueID, err, true)
		}
		return comments, err
	}

	if err != nil {
		c.logGetCommentsFailure(issueID, err, false)
	}

	return comments, err
}

func normalizeCommentID(raw any) (string, error) {
	switch v := raw.(type) {
	case nil:
		return "", errors.New("comment id is NULL")
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int8:
		return strconv.FormatInt(int64(v), 10), nil
	case int16:
		return strconv.FormatInt(int64(v), 10), nil
	case int32:
		return strconv.FormatInt(int64(v), 10), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case uint:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(v), 10), nil
	case uint64:
		return strconv.FormatUint(v, 10), nil
	default:
		return "", fmt.Errorf("unsupported comment id type %T", raw)
	}
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
	if reconnectErr != nil {
		c.logReconnectFailure(reconnectErr)
	}

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
		if c.lastConnectivityStateChange.IsZero() {
			c.lastConnectivityStateChange = time.Now()
		}
	}
	if c.connectivityBroker == nil {
		c.connectivityBroker = pubsub.NewBroker[appbeads.ConnectivityEvent]()
	}
	if c.connectivityState == state {
		c.lastConnectivityErr = err
		c.mu.Unlock()
		return
	}
	c.connectivityState = state
	c.lastConnectivityStateChange = time.Now()
	c.lastConnectivityErr = err
	broker := c.connectivityBroker
	c.mu.Unlock()

	diagnostics := c.DiagnosticContext()

	broker.Publish(pubsub.UpdatedEvent, appbeads.ConnectivityEvent{State: state, Err: err, Diagnostics: &diagnostics})
}

func (c *DoltClient) reconnect() error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	resolved, err := resolveConnectionDetailsWithSource(c.beadsDir)
	if err != nil {
		return err
	}

	newClient, delegatedStartAttempted, err := connectOrDelegateStartup(c.beadsDir, resolved)
	if err != nil {
		return err
	}

	newDB := newClient.DB()

	c.mu.Lock()
	oldDB := c.db
	c.db = newDB
	c.details = resolved.details
	c.portSource = resolved.portSource
	if delegatedStartAttempted {
		c.delegatedStartAttempted = true
	}
	c.mu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}

	return nil
}

func suggestionForError(err error) string {
	if err == nil {
		return ""
	}

	if appbeads.IsProjectMismatchError(err) || appbeads.IsMissingDatabaseError(err) {
		return "Run 'bd bootstrap' to repair project configuration and database wiring, then retry."
	}
	if appbeads.IsCircuitBreakerError(err) {
		return "Run 'bd bootstrap' to repair runtime state; if needed, restart the Dolt server and retry."
	}
	if appbeads.IsDoltPanicError(err) {
		return "Restart the Dolt server for this project, then run 'bd bootstrap' and retry."
	}
	if strings.Contains(strings.ToLower(err.Error()), "dolt-server.port") {
		return "Run 'bd bootstrap' to refresh server connection metadata, then retry Perles."
	}
	if appbeads.IsMissingTableError(err) || appbeads.IsMissingColumnError(err) || IsCompatibilityError(err) {
		return "Run 'bd bootstrap' to apply/repair the beads v1 schema, then retry Perles."
	}
	if appbeads.IsRecoverableConnectivityError(err) || IsServerStartupError(err) {
		return "Ensure this project is using beads v1 server mode and run 'bd bootstrap'; if the server is stopped, run 'bd dolt start'."
	}

	return ""
}

func startupErrorContext(err error, fallback ConnectionDetails) (ConnectionDetails, string) {
	details := fallback
	suggestion := suggestionForError(err)

	var startupErr *StartupError
	if errors.As(err, &startupErr) {
		if startupErr.Details != nil {
			details = *startupErr.Details
		}
		if startupErr.Suggestion != "" {
			suggestion = startupErr.Suggestion
		}
	}

	return details, suggestion
}

func startupLogFields(operation string, details ConnectionDetails, err error, suggestion string) []any {
	fields := []any{
		"operation", operation,
		"host", details.Host,
		"port", details.Port,
		"database", details.Database,
	}

	if suggestion != "" {
		fields = append(fields, "suggestion", suggestion)
	}
	if code, ok := appbeads.ExtractMySQLErrorCode(err); ok {
		fields = append(fields, "mysql_error_code", code)
	}
	if appbeads.IsMissingDatabaseError(err) {
		fields = append(fields, "missing_database", true)
	}
	if err != nil {
		fields = append(fields, "error", err)
	}

	return fields
}

func logStartupFailure(operation string, err error, fallback ConnectionDetails) {
	if err == nil {
		return
	}

	details, suggestion := startupErrorContext(err, fallback)
	log.Error(log.CatDB, "Dolt startup failed", startupLogFields(operation, details, err, suggestion)...)
}

func (c *DoltClient) logGetCommentsFailure(issueID string, err error, reconnectAttempted bool) {
	diagnostics := c.DiagnosticContext()
	fields := []any{
		"operation", "get_comments",
		"issue_id", issueID,
		"host", diagnostics.Host,
		"port", diagnostics.Port,
		"database", diagnostics.Database,
		"reconnect_attempted", reconnectAttempted,
		"connectivity_state", diagnostics.LastState,
	}
	if diagnostics.Suggestion != "" {
		fields = append(fields, "suggestion", diagnostics.Suggestion)
	}
	if code, ok := appbeads.ExtractMySQLErrorCode(err); ok {
		fields = append(fields, "mysql_error_code", code)
	}
	if appbeads.IsMissingDatabaseError(err) {
		fields = append(fields, "missing_database", true)
	}
	fields = append(fields, "error", err)

	log.Error(log.CatDB, "GetComments failed", fields...)
}

func (c *DoltClient) logReconnectFailure(err error) {
	diagnostics := c.DiagnosticContext()
	fields := []any{
		"operation", "reconnect",
		"host", diagnostics.Host,
		"port", diagnostics.Port,
		"database", diagnostics.Database,
		"connectivity_state", diagnostics.LastState,
		"state_transition", "degraded",
	}
	if diagnostics.Suggestion != "" {
		fields = append(fields, "suggestion", diagnostics.Suggestion)
	}
	if code, ok := appbeads.ExtractMySQLErrorCode(err); ok {
		fields = append(fields, "mysql_error_code", code)
	}
	if appbeads.IsMissingDatabaseError(err) {
		fields = append(fields, "missing_database", true)
	}
	fields = append(fields, "error", err)

	log.Error(log.CatDB, "Reconnect exhausted; connectivity degraded", fields...)
}

func enrichStartupError(err error, details ConnectionDetails) error {
	if err == nil {
		return nil
	}

	var startupErr *StartupError
	if errors.As(err, &startupErr) {
		if startupErr.Kind == StartupErrorKindServerStartup && startupErr.Details == nil {
			startupErr.Details = copyConnectionDetails(details)
		}
		if startupErr.Suggestion == "" {
			startupErr.Suggestion = suggestionForError(startupErr.Err)
		}
		return err
	}

	return &StartupError{
		Kind:       StartupErrorKindServerStartup,
		Err:        err,
		Details:    copyConnectionDetails(details),
		Suggestion: suggestionForError(err),
	}
}

func ResolveConnectionDetails(beadsDir string) (ConnectionDetails, error) {
	resolved, err := resolveConnectionDetailsWithSource(beadsDir)
	if err != nil {
		return ConnectionDetails{}, err
	}

	return resolved.details, nil
}

func resolveConnectionDetailsWithSource(beadsDir string) (resolvedConnectionDetails, error) {
	metadataPath := filepath.Join(beadsDir, "metadata.json")
	metadataBytes, err := os.ReadFile(metadataPath) //nolint:gosec // metadataPath is a fixed file under the resolved .beads directory
	if err != nil {
		// no_beads: repository is missing required beads metadata.
		return resolvedConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("reading beads metadata %q: %w", metadataPath, err),
		}
	}

	var metadata beadsMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		// no_beads: beads metadata exists but is invalid/unreadable.
		return resolvedConnectionDetails{}, &StartupError{
			Kind: StartupErrorKindNoBeads,
			Err:  fmt.Errorf("parsing beads metadata %q: %w", metadataPath, err),
		}
	}

	if metadata.Backend != "dolt" {
		// no_beads: backend is not supported by this Dolt client.
		return resolvedConnectionDetails{}, &StartupError{
			Kind:       StartupErrorKindNoBeads,
			Err:        fmt.Errorf("unsupported beads backend %q; expected dolt", metadata.Backend),
			Suggestion: "Perles supports beads v1+ projects backed by Dolt. Run 'bd bootstrap' in a Dolt-backed beads project.",
		}
	}
	if metadata.DoltMode != "server" {
		// no_beads: project is not configured for Dolt server mode.
		return resolvedConnectionDetails{}, &StartupError{
			Kind:       StartupErrorKindNoBeads,
			Err:        fmt.Errorf("unsupported dolt mode %q; expected server", metadata.DoltMode),
			Suggestion: fmt.Sprintf("Perles currently supports beads v1+ only in dolt_mode=server (detected %q). Reconfigure to server mode, then run 'bd bootstrap'.", metadata.DoltMode),
		}
	}
	if strings.TrimSpace(metadata.DoltDatabase) == "" {
		// no_beads: metadata is missing required connection fields.
		return resolvedConnectionDetails{}, &StartupError{
			Kind:       StartupErrorKindNoBeads,
			Err:        errors.New("missing dolt_database in beads metadata"),
			Suggestion: "Run 'bd bootstrap' to regenerate missing beads metadata for this project.",
		}
	}

	host, cfgPort, err := readDoltConfig(filepath.Join(beadsDir, "dolt", "config.yaml"))
	if err != nil {
		// no_beads: connection config is missing/invalid.
		return resolvedConnectionDetails{}, &StartupError{
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
	port, portSource, err := readDoltPort(filepath.Join(beadsDir, "dolt-server.port"), cfgPort)
	if err != nil {
		// server_startup: port file parse/validation/read failures indicate
		// startup/runtime state, not project identity.
		return resolvedConnectionDetails{}, &StartupError{
			Kind:       StartupErrorKindServerStartup,
			Err:        err,
			Details:    copyConnectionDetails(configDetails),
			Suggestion: suggestionForError(err),
		}
	}
	return resolvedConnectionDetails{
		details: ConnectionDetails{
			Host:     configDetails.Host,
			Port:     port,
			Database: configDetails.Database,
		},
		portSource: portSource,
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

func readDoltPort(path string, fallback int) (int, string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // path points to the tracked Dolt port file under the resolved .beads directory
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, "config.yaml", nil
		}
		return 0, "", fmt.Errorf("reading dolt server port file %q: %w", path, err)
	}

	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return fallback, "config.yaml", nil
	}

	port, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, "", fmt.Errorf("parsing dolt server port file %q: %w", path, err)
	}

	if port <= 0 {
		return 0, "", fmt.Errorf("invalid dolt server port %d in %q", port, path)
	}

	return port, "dolt-server.port", nil
}
