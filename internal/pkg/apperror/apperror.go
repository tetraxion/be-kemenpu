// Package apperror defines application errors that map to an HTTP status.
package apperror

import (
	"context"
	"errors"
	"net/http"
)

type Kind int

const (
	KindInternal Kind = iota
	KindInvalid
	KindNotFound
	KindMethodNotAllowed
	// 422
	KindUnprocessable
	// 503
	KindUnavailable
	// 504
	KindTimeout
)

// Error carries a stable code for clients and a message that is safe to show.
type Error struct {
	Kind    Kind
	Code    string
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// Is matches by code, so errors.Is works on a copy made by Wrap.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Wrap returns a copy. The cause only goes to the log.
func (e *Error) Wrap(cause error) *Error {
	c := *e
	c.Err = cause
	return &c
}

func Invalid(code, msg string) *Error  { return &Error{Kind: KindInvalid, Code: code, Message: msg} }
func NotFound(code, msg string) *Error { return &Error{Kind: KindNotFound, Code: code, Message: msg} }
func MethodNotAllowed(code, msg string) *Error {
	return &Error{Kind: KindMethodNotAllowed, Code: code, Message: msg}
}
func Unprocessable(code, msg string) *Error {
	return &Error{Kind: KindUnprocessable, Code: code, Message: msg}
}
func Unavailable(code, msg string) *Error {
	return &Error{Kind: KindUnavailable, Code: code, Message: msg}
}
func Timeout(code, msg string) *Error { return &Error{Kind: KindTimeout, Code: code, Message: msg} }

var (
	errRequestTimeout   = Timeout("REQUEST_TIMEOUT", "permintaan melebihi batas waktu")
	errRequestCancelled = Unavailable("REQUEST_CANCELLED", "permintaan dibatalkan sebelum selesai")
)

// From maps a deadline or cancellation to a timeout, any other unknown error to internal.
func From(err error) *Error {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errRequestTimeout.Wrap(err)
	case errors.Is(err, context.Canceled):
		return errRequestCancelled.Wrap(err)
	}
	return &Error{Kind: KindInternal, Code: "INTERNAL_ERROR", Message: "terjadi kesalahan pada server", Err: err}
}

func HTTPStatus(kind Kind) int {
	switch kind {
	case KindInvalid:
		return http.StatusBadRequest
	case KindNotFound:
		return http.StatusNotFound
	case KindMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case KindUnprocessable:
		return http.StatusUnprocessableEntity
	case KindUnavailable:
		return http.StatusServiceUnavailable
	case KindTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
