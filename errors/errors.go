package errors

import (
	stderrors "errors"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

// Other non-HTTP errors
var (
	ErrConnection  = errors.New("connection failed")
	ErrClosed      = errors.New("closed")
	ErrUnsupported = errors.New("unsupported")
)

var (
	New          = errors.New
	Errorf       = errors.Errorf
	Wrap         = errors.Wrap
	Wrapf        = errors.Wrapf
	WithStack    = errors.WithStack
	WithMessage  = errors.WithMessage
	WithMessagef = errors.WithMessagef
	Cause        = errors.Cause
	Join         = stderrors.Join
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
// Returns the wrapper error to maintain proper chain traversal.
// Use Cause() for direct access to the root cause.
func (w *withCause) Unwrap() error { return w.error }

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
