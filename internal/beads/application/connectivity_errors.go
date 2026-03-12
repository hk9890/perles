package application

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"strings"
	"syscall"

	mysql "github.com/go-sql-driver/mysql"
)

// Reconnector can attempt to recover the backing DB connection after
// recoverable connectivity failures.
type Reconnector interface {
	ReconnectIfRecoverable(err error) (attempted bool, reconnectErr error)
}

// IsRecoverableConnectivityError returns true for transient connectivity
// failures where a reconnect+single retry may succeed.
func IsRecoverableConnectivityError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, driver.ErrBadConn) || errors.Is(err, io.EOF) {
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
		"server has gone away",
		"invalid connection",
		"bad connection",
		"use of closed network connection",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}

	return false
}
