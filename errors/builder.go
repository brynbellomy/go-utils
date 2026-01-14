package errors

import "fmt"

// Builder provides a fluent interface for constructing errors with properties and fields.
// All Builder methods safely handle nil receivers, returning nil without panicking.
// This allows safe chaining even when the initial error is nil.
type Builder struct {
	error
}

// With wraps an existing error with an optional message and returns a Builder.
// Returns nil if parent is nil, enabling safe error handling without explicit nil checks.
// If args are provided, the first must be a string (optionally with format args).
func With(parent error, args ...any) *Builder {
	if parent == nil {
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
// Accepts either a string (to create a new error) or an error (to wrap).
func WithNew(parentOrMsg any, args ...any) *Builder {
	var err error
	switch x := parentOrMsg.(type) {
	case string:
		err = New(x)
	case error:
		err = x
	default:
		panic(fmt.Sprintf("invariant violation: got %T, expected error or string", parentOrMsg))
	}
	return &Builder{error: err}
}

// Err returns the underlying error from the Builder.
// Returns nil if the Builder is nil.
func (b *Builder) Err() error {
	if b == nil {
		return nil
	}
	return b.error
}

// Unwrap returns the underlying error, implementing the Go 1.13+ error unwrapping interface.
// This allows functions like GetStatusCode to traverse through Builder instances.
func (b *Builder) Unwrap() error {
	if b == nil {
		return nil
	}
	return b.error
}

func (b *Builder) Wrap(msg string) *Builder {
	if b == nil {
		return nil
	}
	b.error = Wrap(b.error, msg)
	return b
}

func (b *Builder) Wrapf(msg string, args ...any) *Builder {
	if b == nil {
		return nil
	}
	b.error = Wrapf(b.error, msg, args...)
	return b
}

func (b *Builder) Cause(cause error) *Builder {
	if b == nil {
		return nil
	}
	b.error = WithCause(b.error, cause)
	return b
}

func (b *Builder) Stack() *Builder {
	if b == nil {
		return nil
	}
	b.error = WithStack(b.error)
	return b
}

// Set adds properties and/or fields to the error in a single call.
// Accepts Fault, StatusCode, Retryability, Fields, and individual field key-value pairs.
// All items are stored in a single metadata layer, making everything accessible.
// Returns nil if Builder is nil.
func (b *Builder) Set(things ...any) *Builder {
	if b == nil {
		return nil
	}
	b.error = WithMetadata(b.error, things...)
	return b
}
