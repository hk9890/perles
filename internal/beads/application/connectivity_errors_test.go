package application

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestIsRecoverableConnectivityError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "sql err conn done", err: sql.ErrConnDone, want: true},
		{name: "driver bad conn", err: driver.ErrBadConn, want: true},
		{name: "io eof", err: io.EOF, want: true},
		{name: "io unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "net op refused", err: &net.OpError{Err: syscall.ECONNREFUSED}, want: true},
		{name: "net op reset", err: &net.OpError{Err: syscall.ECONNRESET}, want: true},
		{name: "net op broken pipe", err: &net.OpError{Err: syscall.EPIPE}, want: true},
		{name: "mysql server gone", err: &mysql.MySQLError{Number: 2006, Message: "server has gone away"}, want: true},
		{name: "mysql lost connection", err: &mysql.MySQLError{Number: 2013, Message: "lost connection"}, want: true},
		{name: "string invalid connection", err: errors.New("invalid connection"), want: true},
		{name: "string unexpected eof", err: errors.New("unexpected EOF"), want: true},
		{name: "string connection already closed", err: errors.New("sql: connection is already closed"), want: true},
		{name: "string closed network", err: errors.New("use of closed network connection"), want: true},
		{name: "wrapped refused", err: fmt.Errorf("query failed: %w", &net.OpError{Err: syscall.ECONNREFUSED}), want: true},
		{name: "circuit breaker not recoverable", err: errors.New("circuit breaker open"), want: false},
		{name: "no such database not recoverable", err: errors.New("Error 1049: no such database: project-foo"), want: false},
		{name: "project mismatch not recoverable", err: errors.New("PROJECT IDENTITY MISMATCH: expected abc got def"), want: false},
		{name: "syntax error", err: errors.New("Error 1064: You have an error in your SQL syntax"), want: false},
		{name: "permission error", err: errors.New("access denied for user root@localhost"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsRecoverableConnectivityError(tt.err))
		})
	}
}

func TestIsProjectMismatchError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "project identity mismatch lower", err: errors.New("project identity mismatch for workspace"), want: true},
		{name: "project identity mismatch upper", err: errors.New("PROJECT IDENTITY MISMATCH"), want: true},
		{name: "wrong database", err: errors.New("wrong database selected for current project"), want: true},
		{name: "wrapped mismatch", err: fmt.Errorf("query failed: %w", errors.New("project identity mismatch: expected x")), want: true},
		{name: "near miss project identity", err: errors.New("project identity check failed"), want: false},
		{name: "near miss wrong db", err: errors.New("wrong db selected"), want: false},
		{name: "unrelated", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsProjectMismatchError(tt.err))
		})
	}
}

func TestIsCircuitBreakerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "circuit breaker", err: errors.New("circuit breaker tripped"), want: true},
		{name: "circuit_breaker", err: errors.New("state=circuit_breaker"), want: true},
		{name: "breaker open", err: errors.New("breaker open for endpoint"), want: true},
		{name: "wrapped breaker", err: fmt.Errorf("dial failed: %w", errors.New("circuit breaker open")), want: true},
		{name: "near miss circuit", err: errors.New("circuit is healthy"), want: false},
		{name: "near miss breaker", err: errors.New("breaker half-open transition"), want: false},
		{name: "unrelated", err: errors.New("project identity mismatch"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsCircuitBreakerError(tt.err))
		})
	}
}

func TestIsDoltPanicError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "runtime error", err: errors.New("runtime error: index out of range"), want: true},
		{name: "panic", err: errors.New("panic: unexpected condition"), want: true},
		{name: "nil pointer dereference", err: errors.New("invalid memory address or nil pointer dereference"), want: true},
		{name: "concurrent map", err: errors.New("fatal error: concurrent map writes"), want: true},
		{name: "wrapped panic", err: fmt.Errorf("backend crashed: %w", errors.New("panic: concurrent map read and map write")), want: true},
		{name: "near miss runtime", err: errors.New("runtime configuration changed"), want: false},
		{name: "near miss map", err: errors.New("mapping updated concurrently"), want: false},
		{name: "unrelated", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsDoltPanicError(tt.err))
		})
	}
}

func TestIsMissingDatabaseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "mysql 1049", err: &mysql.MySQLError{Number: 1049, Message: "Unknown database 'perles'"}, want: true},
		{name: "unknown database text", err: errors.New("Error 1049 (42000): Unknown database 'perles'"), want: true},
		{name: "no such database text", err: errors.New("no such database: perles"), want: true},
		{name: "database not found", err: errors.New("database not found: perles"), want: true},
		{name: "wrapped", err: fmt.Errorf("query failed: %w", &mysql.MySQLError{Number: 1049, Message: "unknown database"}), want: true},
		{name: "different mysql code", err: &mysql.MySQLError{Number: 1064, Message: "syntax error"}, want: false},
		{name: "unrelated", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsMissingDatabaseError(tt.err))
		})
	}
}

func TestExtractMySQLErrorCode(t *testing.T) {
	code, ok := ExtractMySQLErrorCode(&mysql.MySQLError{Number: 2013, Message: "lost connection"})
	require.True(t, ok)
	require.Equal(t, uint16(2013), code)

	wrapped := fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1049, Message: "unknown database"})
	code, ok = ExtractMySQLErrorCode(wrapped)
	require.True(t, ok)
	require.Equal(t, uint16(1049), code)

	code, ok = ExtractMySQLErrorCode(errors.New("plain error"))
	require.False(t, ok)
	require.Equal(t, uint16(0), code)
}
