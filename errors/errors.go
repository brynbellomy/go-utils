package errors

import (
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

// StatusCoder represents an error with an associated HTTP status code
type StatusCoder struct {
	Code    int
	Message string
}

func (e *StatusCoder) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func NewStatusCoder(code int, message string) *StatusCoder {
	return &StatusCoder{
		Code:    code,
		Message: message,
	}
}

func AsStatusCoder(err error) (*StatusCoder, bool) {
	var httpErr *StatusCoder
	if errors.As(err, &httpErr) {
		return httpErr, true
	}
	return nil, false
}

func IsStatusCoder(err error, code ...int) bool {
	var httpErr *StatusCoder
	if errors.As(err, &httpErr) {
		if len(code) == 0 {
			return true
		}
		for _, c := range code {
			if httpErr.Code == c {
				return true
			}
		}
	}
	return false
}

// HTTP-style errors (useful elsewhere too)
var (
	ErrBadRequest          = NewStatusCoder(http.StatusBadRequest, "bad Request")
	ErrUnauthorized        = NewStatusCoder(http.StatusUnauthorized, "unauthorized")
	ErrForbidden           = NewStatusCoder(http.StatusForbidden, "forbidden")
	ErrNotFound            = NewStatusCoder(http.StatusNotFound, "not found")
	ErrTooManyRequests     = NewStatusCoder(http.StatusTooManyRequests, "too many requests")
	ErrInternalServerError = NewStatusCoder(http.StatusInternalServerError, "internal server error")
	ErrUnimplemented       = NewStatusCoder(http.StatusNotImplemented, "not implemented")
	ErrServiceUnavailable  = NewStatusCoder(http.StatusServiceUnavailable, "service unavailable")
)

// Other non-HTTP errors
var (
	ErrConnection  = errors.New("connection failed")
	ErrClosed      = errors.New("closed")
	ErrUnsupported = errors.New("unsupported")
)

var (
	New       = errors.New
	Errorf    = errors.Errorf
	Wrap      = errors.Wrap
	Wrapf     = errors.Wrapf
	WithStack = errors.WithStack
	Cause     = errors.Cause
)

func Annotate(err *error, msg string, args ...any) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

func AddStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}

func OneOf(received error, errs ...error) bool {
	for _, err := range errs {
		if Cause(received) == err {
			return true
		}
	}
	return false
}

func WithCause(err error, cause error) error {
	return &withCause{err, cause}
}

type withCause struct {
	error
	cause error
}

func (w *withCause) Error() string { return w.error.Error() + ": " + w.cause.Error() }

func (w *withCause) Cause() error { return w.cause }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withCause) Unwrap() error { return w.cause }

func (w *withCause) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			fmt.Fprintf(s, "%+v\n", w.Cause())
			io.WriteString(s, w.error.Error())
			return
		}
		fallthrough
	case 's', 'q':
		io.WriteString(s, w.Error())
	}
}
