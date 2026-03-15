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

// IsRecoverableConnectivityError returns true for transient connectivity
// failures where a reconnect+single retry may succeed.
func IsRecoverableConnectivityError(err error) bool {
	if err == nil {
		return false
	}

	// Explicitly do not retry known non-transient categories.
	if IsCircuitBreakerError(err) || IsProjectMismatchError(err) || hasErrorSubstring(err, "no such database") {
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

	msg := strings.ToLower(err.Error())
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
	for _, needle := range needles {
		if strings.Contains(msg, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}
