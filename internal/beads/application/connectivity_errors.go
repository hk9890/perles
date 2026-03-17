package application

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/hk9890/perles/internal/pubsub"
)

// Reconnector can attempt to recover the backing DB connection after
// recoverable connectivity failures.
type Reconnector interface {
	ReconnectIfRecoverable(err error) (attempted bool, reconnectErr error)
}

// ConnectivityState represents the backend connectivity state exposed to the UI.
type ConnectivityState string

const (
	ConnectivityStateHealthy      ConnectivityState = "healthy"
	ConnectivityStateReconnecting ConnectivityState = "reconnecting"
	ConnectivityStateDegraded     ConnectivityState = "degraded"
)

// ConnectivityEvent is emitted when backend connectivity state changes.
type ConnectivityEvent struct {
	State       ConnectivityState
	Err         error
	Diagnostics *DiagnosticContext
}

// DiagnosticContext is structured connectivity context for UI diagnostics.
type DiagnosticContext struct {
	Host                    string
	Port                    int
	Database                string
	LastState               ConnectivityState
	LastStateChange         time.Time
	DelegatedStartAttempted bool
	PortSource              string
	Suggestion              string
}

// ConnectivityObserver exposes backend connectivity state transitions.
type ConnectivityObserver interface {
	ConnectivityState() ConnectivityState
	ConnectivityBroker() *pubsub.Broker[ConnectivityEvent]
}

// IsProjectMismatchError returns true when the selected DB does not match
// the expected project identity.
func IsProjectMismatchError(err error) bool {
	return hasErrorSubstring(err,
		"project identity mismatch",
		"wrong database",
	)
}

// IsCircuitBreakerError returns true when the backend is refusing work due to
// an open circuit breaker.
func IsCircuitBreakerError(err error) bool {
	return hasErrorSubstring(err,
		"circuit breaker",
		"circuit_breaker",
		"breaker open",
	)
}

// IsDoltPanicError returns true for upstream Dolt panic/crash indicators.
func IsDoltPanicError(err error) bool {
	return hasErrorSubstring(err,
		"runtime error",
		"panic",
		"nil pointer dereference",
		"concurrent map",
	)
}

// IsMissingDatabaseError returns true when the configured database does not
// exist in the target Dolt/MySQL server.
func IsMissingDatabaseError(err error) bool {
	if err == nil {
		return false
	}

	if code, ok := ExtractMySQLErrorCode(err); ok && code == 1049 {
		return true
	}

	return hasErrorSubstring(err,
		"unknown database",
		"no such database",
		"database not found",
	)
}

// ExtractMySQLErrorCode returns the wrapped MySQL error code, when present.
func ExtractMySQLErrorCode(err error) (uint16, bool) {
	if err == nil {
		return 0, false
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number, true
	}

	return 0, false
}

// IsRecoverableConnectivityError returns true for transient connectivity
// failures where a reconnect+single retry may succeed.
func IsRecoverableConnectivityError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	// Explicitly do not retry known non-transient categories.
	if isCircuitBreakerErrorMessage(msg) || isProjectMismatchErrorMessage(msg) || isMissingDatabaseError(err, msg) {
		return false
	}

	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if errors.Is(netErr.Err, syscall.ECONNREFUSED) || errors.Is(netErr.Err, syscall.ECONNRESET) || errors.Is(netErr.Err, syscall.EPIPE) {
			return true
		}
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		// 2006: MySQL server has gone away, 2013: lost connection.
		if mysqlErr.Number == 2006 || mysqlErr.Number == 2013 {
			return true
		}
	}

	for _, needle := range []string{
		"connection refused",
		"broken pipe",
		"connection reset",
		"unexpected eof",
		"server has gone away",
		"invalid connection",
		"bad connection",
		"connection is already closed",
		"database is closed",
		"use of closed network connection",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}

	return false
}

func hasErrorSubstring(err error, needles ...string) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return hasSubstringInLoweredMessage(msg, needles...)
}

func isProjectMismatchErrorMessage(msg string) bool {
	return hasSubstringInLoweredMessage(msg,
		"project identity mismatch",
		"wrong database",
	)
}

func isCircuitBreakerErrorMessage(msg string) bool {
	return hasSubstringInLoweredMessage(msg,
		"circuit breaker",
		"circuit_breaker",
		"breaker open",
	)
}

func isMissingDatabaseError(err error, msg string) bool {
	if code, ok := ExtractMySQLErrorCode(err); ok && code == 1049 {
		return true
	}

	return hasSubstringInLoweredMessage(msg,
		"unknown database",
		"no such database",
		"database not found",
	)
}

func hasSubstringInLoweredMessage(msg string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(msg, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}
