package database

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestIsUnavailable(t *testing.T) {
	unavailable := []error{
		&pgconn.ConnectError{},
		&net.OpError{Op: "dial", Err: errors.New("connection refused")},
		&pgconn.PgError{Code: "08006"}, // connection failure
		&pgconn.PgError{Code: "57P01"}, // admin shutdown
		&pgconn.PgError{Code: "53300"}, // too many connections
	}
	for _, err := range unavailable {
		assert.True(t, IsUnavailable(fmt.Errorf("asset: list: %w", err)), "%v", err)
	}

	other := []error{
		errors.New("something else"),
		&pgconn.PgError{Code: "23505"}, // unique violation
		&pgconn.PgError{Code: "42P01"}, // undefined table
		nil,
	}
	for _, err := range other {
		assert.False(t, IsUnavailable(err), "%v", err)
	}
}
