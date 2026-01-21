package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"slices"

	"github.com/pkg/errors"
)

// Common sentinel errors for various failure scenarios.
var (
	// ErrConnection indicates that a network or service connection failed.
	ErrConnection = errors.New("connection failed")
	// ErrClosed indicates that an operation was attempted on a closed resource.
	ErrClosed = errors.New("closed")
	// ErrUnsupported indicates that a requested feature or operation is not supported.
	ErrUnsupported = errors.New("unsupported")
)

// Re-exported functions from github.com/pkg/errors and standard library for convenience.
var (
	// New returns an error that formats as the given text. Each call to New returns
	// a distinct error value even if the text is identical.
	New = errors.New
	// Errorf formats according to a format specifier and returns the string as a
	// value that satisfies error.
	Errorf = errors.Errorf
	// Wrap returns an error annotating err with a stack trace at the point Wrap is called,
	// and the supplied message. If err is nil, Wrap returns nil.
	Wrap = errors.Wrap
	// Wrapf returns an error annotating err with a stack trace at the point Wrapf is called,
	// and the format specifier. If err is nil, Wrapf returns nil.
	Wrapf = errors.Wrapf
	// WithStack annotates err with a stack trace at the point WithStack was called.
	// If err is nil, WithStack returns nil.
	WithStack = errors.WithStack
	// WithMessage annotates err with a new message. If err is nil, WithMessage returns nil.
	WithMessage = errors.WithMessage
	// WithMessagef annotates err with the format specifier. If err is nil, WithMessagef returns nil.
	WithMessagef = errors.WithMessagef
	// Cause returns the underlying cause of the error, if possible. An error value has a cause
	// if it implements the causer interface.
	Cause = errors.Cause
	// Join returns an error that wraps the given errors. Any nil error values are discarded.
	// Join returns nil if every value in errs is nil.
	Join = stderrors.Join
)

// Annotate wraps the error pointed to by err with the formatted message if err is non-nil.
// This is useful for defer statements where you want to add context to any error returned.
//
// Example usage:
//
//	func doSomething() (err error) {
//	    defer Annotate(&err, "failed to do something")
//	    // ... do work that might return an error
//	}
func Annotate(err *error, msg string, args ...any) {
	if *err != nil {
		*err = errors.Wrapf(*err, msg, args...)
	}
}

// AddStack adds a stack trace to the error pointed to by err if err is non-nil.
// This is useful for defer statements where you want to capture the stack trace at the
// point where the function returns.
//
// Example usage:
//
//	func doSomething() (err error) {
//	    defer AddStack(&err)
//	    // ... do work that might return an error
//	}
func AddStack(err *error) {
	if *err != nil {
		*err = errors.WithStack(*err)
	}
}

// OneOf returns true if the root cause of the received error matches any of the provided errors.
// It uses Cause to unwrap the received error to its root cause before comparison.
//
// Example usage:
//
//	if OneOf(err, io.EOF, io.ErrUnexpectedEOF) {
//	    // handle EOF-related errors
//	}
func OneOf(received error, errs ...error) bool {
	return slices.Contains(errs, Cause(received))
}

// WithCause wraps an error with an explicit root cause. This is useful when you want to
// create a new error message but preserve the original error as the cause.
// The returned error implements the Cause() interface for accessing the root cause,
// and Unwrap() for Go 1.13+ error chain compatibility.
//
// Example usage:
//
//	if err := validate(input); err != nil {
//	    return WithCause(errors.New("invalid request"), err)
//	}
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
