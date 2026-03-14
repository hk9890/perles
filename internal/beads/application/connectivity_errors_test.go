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
		{name: "syntax error", err: errors.New("Error 1064: You have an error in your SQL syntax"), want: false},
		{name: "permission error", err: errors.New("access denied for user root@localhost"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsRecoverableConnectivityError(tt.err))
		})
	}
}
