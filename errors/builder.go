package errors

import (
	"fmt"
	"io"
)

// Builder provides a fluent interface for constructing errors with properties and fields.
// All Builder methods safely handle nil receivers, returning nil without panicking.
// This allows safe chaining even when the initial error is nil.
// Builder methods never modify their receiver; each returns a new Builder, so a
// Builder can be shared or stored without later chains altering it.
// A non-nil Builder formats, unwraps, and reports its cause exactly as the error
// it holds does, so it can be returned as an error without calling Err.
type Builder struct {
	error
}

// With wraps an existing error with an optional message and returns a Builder.
// Returns nil if parent is nil or typed-nil, enabling safe error handling without explicit nil checks.
// If args are provided, the first must be a string (optionally with format args).
func With(parent error, args ...any) *Builder {
	if isNilError(parent) {
		return nil
	} else if len(args) == 0 {
		return &Builder{error: parent}
	}

	if msg, isStr := args[0].(string); !isStr {
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", msg))
	} else {
		if len(args) > 1 {
			msg = fmt.Sprintf(msg, args[1:]...)
		}
		return &Builder{error: Wrap(parent, msg)}
	}
}

// WithNew creates a new error or wraps an existing one, returning a Builder.
// Accepts either a string (to create a new error, formatted with args when any
// are given) or an error (to wrap; args are ignored).
// Returns nil if the provided error is nil or typed-nil.
func WithNew(parentOrMsg any, args ...any) *Builder {
	var err error
	switch x := parentOrMsg.(type) {
	case string:
		if len(args) > 0 {
			x = fmt.Sprintf(x, args...)
		}
		err = New(x)
	case error:
		if isNilError(x) {
			return nil
		}
		err = x
	default:
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", parentOrMsg))
	}
	return &Builder{error: err}
}

func (b *Builder) Error() string {
	if b == nil || isNilError(b.error) {
		return nilErrorString
	}
	return b.error.Error()
}

// Format formats the Builder exactly as the error it holds, preserving verbose
// (%+v) output such as stack traces.
func (b *Builder) Format(s fmt.State, verb rune) {
	if b == nil || isNilError(b.error) {
		io.WriteString(s, nilErrorString)
		return
	}
	if f, ok := b.error.(fmt.Formatter); ok {
		f.Format(s, verb)
		return
	}
	fmt.Fprintf(s, fmt.FormatString(s, verb), b.error)
}

// Cause returns the underlying error, implementing the causer interface from
// github.com/pkg/errors so that Cause traverses through Builder instances.
func (b *Builder) Cause() error {
	if b == nil {
		return nil
	}
	return normalizeError(b.error)
}

// Err returns the underlying error from the Builder.
// Returns nil if the Builder is nil.
func (b *Builder) Err() error {
	if b == nil {
		return nil
	}
	return normalizeError(b.error)
}

// Unwrap returns the underlying error, implementing the Go 1.13+ error unwrapping interface.
// This allows functions like GetStatusCode to traverse through Builder instances.
func (b *Builder) Unwrap() error {
	if b == nil {
		return nil
	}
	return normalizeError(b.error)
}

func (b *Builder) Wrap(msg string) *Builder {
	if b == nil || isNilError(b.error) {
		return nil
	}
	return &Builder{error: Wrap(b.error, msg)}
}

func (b *Builder) Wrapf(msg string, args ...any) *Builder {
	if b == nil || isNilError(b.error) {
		return nil
	}
	return &Builder{error: Wrapf(b.error, msg, args...)}
}

// CausedBy records cause as the cause of the Builder's error.
func (b *Builder) CausedBy(cause error) *Builder {
	if b == nil || isNilError(b.error) {
		return nil
	}
	return &Builder{error: WithCause(b.error, cause)}
}

func (b *Builder) Stack() *Builder {
	if b == nil || isNilError(b.error) {
		return nil
	}
	return &Builder{error: WithStack(b.error)}
}

// Set adds properties and/or fields to the error in a single call.
// Accepts Fault, StatusCode, Retryability, Fields, and individual field key-value pairs.
// All items are stored in a single metadata layer, making everything accessible.
// Returns nil if Builder is nil.
func (b *Builder) Set(things ...any) *Builder {
	if b == nil || isNilError(b.error) {
		return nil
	}
	return &Builder{error: WithMetadata(b.error, things...)}
}
