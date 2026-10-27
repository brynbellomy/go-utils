package errors

import (
	stderrors "errors"
	"fmt"

	pkgerrors "github.com/pkg/errors"
)

// Common sentinel errors for various failure scenarios.
var (
	// ErrConnection indicates that a network or service connection failed.
	ErrConnection = Sentinel("connection failed")
	// ErrClosed indicates that an operation was attempted on a closed resource.
	ErrClosed = Sentinel("closed")
	// ErrUnsupported indicates that a requested feature or operation is not supported.
	ErrUnsupported = Sentinel("unsupported")
)

// Re-exported functions from github.com/pkg/errors and the standard library.
var (
	// Cause returns the underlying cause of the error, if possible. An error value has a cause
	// if it implements the causer interface; every *Error does.
	Cause = pkgerrors.Cause
	// Join returns an error that wraps the given errors. Any nil error values are discarded.
	// Join returns nil if every value in errs is nil.
	Join = stderrors.Join
	// Is reports whether any error in err's chain matches target. It is the
	// standard library's errors.Is, re-exported so callers do not need a
	// second import to check error identity alongside this package's helpers.
	Is = stderrors.Is
	// As finds the first error in err's chain that matches target, and if one
	// is found, sets target to that error value and returns true. It is the
	// standard library's errors.As, re-exported so callers do not need a
	// second import to unwrap into a concrete error type alongside this
	// package's helpers.
	As = stderrors.As
	// Unwrap returns the result of calling the Unwrap method on err, if err's
	// type contains an Unwrap method returning error. Otherwise, Unwrap
	// returns nil. It is the standard library's errors.Unwrap.
	Unwrap = stderrors.Unwrap
)

// AsType finds the first error in err's chain that is of type E, as by As,
// and returns it.
func AsType[E error](err error) (E, bool) {
	var target E
	ok := stderrors.As(err, &target)
	return target, ok
}

// Annotate wraps the error pointed to by err with the formatted message if err is non-nil,
// as by Wrapf. This is useful for defer statements where you want to add context to any
// error returned. A typed-nil *err is replaced with an untyped nil.
//
// Example usage:
//
//	func doSomething() (err error) {
//	    defer Annotate(&err, "failed to do something")
//	    // ... do work that might return an error
//	}
func Annotate(err *error, msg string, args ...any) {
	if err == nil {
		return
	}
	if isNilError(*err) {
		*err = nil
		return
	}
	*err = &Error{msg: fmt.Sprintf(msg, args...), parent: *err, stack: callersIfNone(*err)}
}

// AddStack wraps the error pointed to by err as by WithStack if err is non-nil.
// This is useful for defer statements where you want to capture the stack trace at the
// point where the function returns. A typed-nil *err is replaced with an untyped nil.
//
// Example usage:
//
//	func doSomething() (err error) {
//	    defer AddStack(&err)
//	    // ... do work that might return an error
//	}
func AddStack(err *error) {
	if err == nil {
		return
	}
	if isNilError(*err) {
		*err = nil
		return
	}
	*err = &Error{parent: *err, stack: callersIfNone(*err)}
}

// OneOf reports whether any of errs matches received, as by errors.Is.
//
// Example usage:
//
//	if OneOf(err, io.EOF, io.ErrUnexpectedEOF) {
//	    // handle EOF-related errors
//	}
func OneOf(received error, errs ...error) bool {
	if isNilError(received) {
		return false
	}
	for _, target := range errs {
		if stderrors.Is(received, target) {
			return true
		}
	}
	return false
}
