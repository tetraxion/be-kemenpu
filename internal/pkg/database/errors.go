package database

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsUnavailable reports a failed connection, a network error or SQLSTATE class 08, 57P and 53.
func IsUnavailable(err error) bool {
	// A request that hit its own deadline also satisfies net.Error; that is
	// the request timing out, not the database being down.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var connectErr *pgconn.ConnectError
	if errors.As(err, &connectErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) == 5 {
		switch pgErr.Code[:2] {
		case "08", "53":
			return true
		}
		return pgErr.Code[:3] == "57P"
	}
	return false
}
