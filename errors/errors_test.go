package errors_test

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"os"
	"testing"

	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/go-utils/errors"
)

func TestBuilderWithStatusCode(t *testing.T) {
	var errfields errors.Fields
	errfields.Add("foo", "bar")
	cause := errors.New("blah")
	baseErr := errors.WithMetadata(errors.New("internal server error"), errors.StatusCode(http.StatusInternalServerError))
	err := errors.With(baseErr, "could not store artifact").Cause(cause).Set(errfields)
	require.Equal(t, http.StatusInternalServerError, errors.GetStatusCode(err))
}

// WithCause Tests

func TestWithCause(t *testing.T) {
	original := pkgerrors.New("original error")
	cause := pkgerrors.New("root cause")

	result := errors.WithCause(original, cause)
	require.NotNil(t, result)

	// Test Error() method
	expected := "original error: root cause"
	require.Equal(t, expected, result.Error())

	// Test Cause() method - provides direct access to root cause
	causer, ok := result.(interface{ Cause() error })
	require.True(t, ok)
	require.Equal(t, cause, causer.Cause())

	// Test Unwrap() method - returns the wrapper to maintain proper chain
	require.Equal(t, original, pkgerrors.Unwrap(result))

	// Can still access cause via Cause() method
	require.Equal(t, cause, causer.Cause())
}

func TestWithCause_Format(t *testing.T) {
	original := pkgerrors.New("original error")
	cause := pkgerrors.New("root cause")
	withCauseErr := errors.WithCause(original, cause)

	tests := []struct {
		name   string
		format string
		want   string
	}{
		{"SimpleString", "%s", "original error: root cause"},
		{"Verb_v", "%v", "original error: root cause"},
		{"Quote", "%q", "original error: root cause"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, withCauseErr)
			require.Equal(t, tt.want, result)
		})
	}
}

func TestWithCause_FormatVerbose(t *testing.T) {
	original := pkgerrors.New("original error")
	cause := pkgerrors.New("root cause")
	withCauseErr := errors.WithCause(original, cause)

	result := fmt.Sprintf("%+v", withCauseErr)
	require.Contains(t, result, "root cause")
	require.Contains(t, result, "original error")
}

// WithMetadata Tests (Fields)

func TestWithMetadata_Fields(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	fields := []any{"key1", "value1", "key2", 42}

	result := errors.WithMetadata(baseErr, fields...)
	require.NotNil(t, result)

	// Test Error() method
	require.Equal(t, "base error", result.Error())

	// Test Unwrap() method
	require.Equal(t, baseErr, pkgerrors.Unwrap(result))

	// Test GetFields() extraction
	extractedFields := errors.GetFields(result).List()
	require.Equal(t, fields, extractedFields)
}

func TestWithMetadata_NewWithFields(t *testing.T) {
	fields := []any{"user_id", 123, "action", "login"}
	result := errors.WithMetadata(errors.New("authentication failed"), fields...)

	require.NotNil(t, result)
	require.Equal(t, "authentication failed", result.Error())

	extractedFields := errors.GetFields(result).List()
	require.Equal(t, fields, extractedFields)
}

func TestWithMetadata_WrapWithFields(t *testing.T) {
	originalErr := pkgerrors.New("database connection failed")
	fields := []any{"host", "localhost", "port", 5432}

	result := errors.WithMetadata(errors.Wrap(originalErr, "failed to connect"), fields...)
	require.NotNil(t, result)
	require.Equal(t, "failed to connect: database connection failed", result.Error())

	extractedFields := errors.GetFields(result).List()
	require.Equal(t, fields, extractedFields)
}

func TestFields_MultipleNesting(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	fields1 := []any{"level", 1, "type", "database"}
	fields2 := []any{"level", 2, "operation", "query"}
	fields3 := []any{"level", 3, "table", "users"}

	// Create nested field errors
	err1 := errors.WithMetadata(baseErr, fields1...)
	err2 := errors.WithMetadata(err1, fields2...)
	err3 := errors.WithMetadata(err2, fields3...)

	// Fields should be collected from all levels
	allFields := errors.GetFields(err3).List()
	expected := append(fields3, append(fields2, fields1...)...)
	require.Equal(t, expected, allFields)
}

func TestFields_NoFields(t *testing.T) {
	regularErr := pkgerrors.New("regular error")
	fields := errors.GetFields(regularErr).List()
	require.Empty(t, fields)
}

func TestFields_EmptyFields(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	result := errors.WithMetadata(baseErr)

	fields := errors.GetFields(result).List()
	require.Empty(t, fields)
}

func TestWithMetadata_NilParent(t *testing.T) {
	fields := []any{"key", "value"}
	result := errors.WithMetadata(nil, fields...)

	require.Nil(t, result)

	extractedFields := errors.GetFields(result).List()
	require.Nil(t, extractedFields)
}

func TestWithMetadata_Format(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		expected string
	}{
		{
			name:     "SimpleFields",
			err:      errors.WithMetadata(errors.New("operation failed"), "user_id", 123, "action", "login"),
			format:   "%s",
			expected: "operation failed user_id=123 action=login",
		},
		{
			name:     "StringWithSpaces",
			err:      errors.WithMetadata(errors.New("request failed"), "method", "GET", "path", "/api/v1/users"),
			format:   "%s",
			expected: "request failed method=GET path=/api/v1/users",
		},
		{
			name:     "StringRequiringQuotes",
			err:      errors.WithMetadata(errors.New("error occurred"), "message", "connection timed out", "code", 500),
			format:   "%s",
			expected: `error occurred message="connection timed out" code=500`,
		},
		{
			name:     "MixedTypes",
			err:      errors.WithMetadata(errors.New("validation error"), "field", "email", "value", "test@example.com", "required", true, "max_length", 255),
			format:   "%s",
			expected: "validation error field=email value=test@example.com required=true max_length=255",
		},
		{
			name:     "VerboseFormat",
			err:      errors.WithMetadata(errors.New("base error"), "key", "value"),
			format:   "%v",
			expected: "base error key=value",
		},
		{
			name:     "QuotedFormat",
			err:      errors.WithMetadata(errors.New("error"), "key", "value"),
			format:   "%q",
			expected: `"error"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf(tt.format, tt.err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestWithMetadata_FormatNested(t *testing.T) {
	baseErr := pkgerrors.New("database error")
	err1 := errors.WithMetadata(baseErr, "table", "users", "operation", "insert")
	err2 := errors.WithMetadata(err1, "user_id", 123, "retry", 3)

	result := fmt.Sprintf("%s", err2)
	// Fields should be collected from all levels
	require.Contains(t, result, "user_id=123")
	require.Contains(t, result, "retry=3")
	require.Contains(t, result, "table=users")
	require.Contains(t, result, "operation=insert")
	require.Contains(t, result, "database error")
}

func TestWithMetadata_FormatVerbosePlus(t *testing.T) {
	baseErr := pkgerrors.New("connection failed")
	fieldErr := errors.WithMetadata(baseErr, "host", "localhost", "port", 5432)

	result := fmt.Sprintf("%+v", fieldErr)
	// Verbose format should include fields
	require.Contains(t, result, "host=localhost")
	require.Contains(t, result, "port=5432")
	// And should include the base error (with stack if pkg/errors added it)
	require.Contains(t, result, "connection failed")
}

// Utility Function Tests

func TestAnnotate(t *testing.T) {
	t.Run("WithError", func(t *testing.T) {
		originalErr := pkgerrors.New("original error")
		err := originalErr

		errors.Annotate(&err, "annotation: %s", "context")
		require.NotEqual(t, originalErr, err)
		require.Contains(t, err.Error(), "annotation: context")
		require.Contains(t, err.Error(), "original error")
	})

	t.Run("WithNilError", func(t *testing.T) {
		var err error
		errors.Annotate(&err, "annotation")
		require.Nil(t, err)
	})
}

func TestAddStack(t *testing.T) {
	t.Run("WithError", func(t *testing.T) {
		originalErr := pkgerrors.New("original error")
		err := originalErr

		errors.AddStack(&err)
		require.NotEqual(t, originalErr, err)

		// Check that stack was added by verifying it implements the interface
		type stackTracer interface {
			StackTrace() pkgerrors.StackTrace
		}
		_, hasStack := err.(stackTracer)
		require.True(t, hasStack)
	})

	t.Run("WithNilError", func(t *testing.T) {
		var err error
		errors.AddStack(&err)
		require.Nil(t, err)
	})
}

func TestOneOf(t *testing.T) {
	err1 := pkgerrors.New("error 1")
	err2 := pkgerrors.New("error 2")
	err3 := pkgerrors.New("error 3")
	wrappedErr1 := pkgerrors.Wrap(err1, "wrapped")

	tests := []struct {
		name     string
		received error
		errors   []error
		expected bool
	}{
		{"MatchFirst", err1, []error{err1, err2, err3}, true},
		{"MatchMiddle", err2, []error{err1, err2, err3}, true},
		{"MatchLast", err3, []error{err1, err2, err3}, true},
		{"NoMatch", pkgerrors.New("different"), []error{err1, err2, err3}, false},
		{"WrappedMatch", wrappedErr1, []error{err1, err2, err3}, true},
		{"EmptyList", err1, []error{}, false},
		{"NilReceived", nil, []error{err1, err2, err3}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.OneOf(tt.received, tt.errors...)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPredefinedNonHTTPErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"Connection", errors.ErrConnection, "connection failed"},
		{"Closed", errors.ErrClosed, "closed"},
		{"Unsupported", errors.ErrUnsupported, "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

func TestReExportedFunctions(t *testing.T) {
	// Test that re-exported functions work correctly
	err1 := errors.New("new error")
	require.Equal(t, "new error", err1.Error())

	err2 := errors.Errorf("formatted error: %d", 42)
	require.Equal(t, "formatted error: 42", err2.Error())

	baseErr := pkgerrors.New("base")
	wrapped := errors.Wrap(baseErr, "wrapped")
	require.Contains(t, wrapped.Error(), "wrapped")
	require.Contains(t, wrapped.Error(), "base")
	require.Equal(t, baseErr, errors.Cause(wrapped))

	wrappedF := errors.Wrapf(baseErr, "wrapped with %s", "format")
	require.Contains(t, wrappedF.Error(), "wrapped with format")
	require.Contains(t, wrappedF.Error(), "base")

	withStack := errors.WithStack(baseErr)
	require.NotEqual(t, baseErr, withStack)
}

// Integration and Edge Case Tests

func TestComplexErrorChain(t *testing.T) {
	// Test combining WithMetadata with StatusCode, WithCause, and fields
	baseErr := pkgerrors.New("database timeout")
	statusErr := errors.WithMetadata(errors.New("internal error"), errors.StatusCode(500))
	causeErr := errors.WithCause(statusErr, baseErr)
	fieldErr := errors.WithMetadata(causeErr, "user_id", 123, "operation", "read")

	// Test error message
	require.Contains(t, fieldErr.Error(), "internal error")
	require.Contains(t, fieldErr.Error(), "database timeout")

	// Test status code extraction
	require.Equal(t, 500, errors.GetStatusCode(fieldErr))

	// Test fields extraction
	fields := errors.GetFields(fieldErr).List()
	require.Equal(t, []any{"user_id", 123, "operation", "read"}, fields)

	// Test cause unwrapping - fieldErr unwraps to causeErr, not baseErr
	require.Equal(t, causeErr, pkgerrors.Unwrap(fieldErr))
}

func TestStandardLibraryCompatibility(t *testing.T) {
	// Test compatibility with standard library pkgerrors.As and pkgerrors.Is
	baseErr := errors.New("not found")
	statusErr := errors.WithMetadata(baseErr, errors.StatusCode(404))
	wrappedErr := pkgerrors.Wrap(statusErr, "wrapped")
	fieldErr := errors.WithMetadata(wrappedErr, "resource", "user")

	// Test status code extraction through wrapper chain
	require.Equal(t, 404, errors.GetStatusCode(fieldErr))
	require.Equal(t, 404, errors.GetStatusCode(wrappedErr))

	// Test pkgerrors.Is behavior - since our errors use standard unwrapping,
	// this should work for direct comparisons
	require.True(t, pkgerrors.Is(wrappedErr, statusErr))
	// And also work for field-wrapped errors due to proper unwrapping chain
	require.True(t, pkgerrors.Is(fieldErr, statusErr))
	require.True(t, pkgerrors.Is(fieldErr, baseErr))
}

func TestNilAndZeroValues(t *testing.T) {
	t.Run("GetStatusCodeNil", func(t *testing.T) {
		// Test getting status code from nil error
		require.Equal(t, 0, errors.GetStatusCode(nil))
	})

	t.Run("GetStatusCodeNoStatusCode", func(t *testing.T) {
		// Test getting status code from error without status code
		err := errors.New("some error")
		require.Equal(t, 0, errors.GetStatusCode(err))
	})

	t.Run("WithCauseNilCause", func(t *testing.T) {
		// Test WithCause with nil cause - this will panic on Error() call
		// so we shouldn't call Error(), just test the structure
		baseErr := pkgerrors.New("base")
		result := errors.WithCause(baseErr, nil)
		require.NotNil(t, result)

		// Test that Cause() returns nil
		causer, ok := result.(interface{ Cause() error })
		require.True(t, ok)
		require.Nil(t, causer.Cause())

		// Test that Unwrap() returns the wrapper (baseErr), not the cause
		require.Equal(t, baseErr, pkgerrors.Unwrap(result))
	})
}

func TestErrorUnwrappingChain(t *testing.T) {
	base := pkgerrors.New("base error")
	level1Err := pkgerrors.New("level 1")
	level2Err := pkgerrors.New("level 2")
	cause1 := errors.WithCause(level1Err, base)
	cause2 := errors.WithCause(level2Err, cause1)
	fields := errors.WithMetadata(cause2, "key", "value")

	// Test unwrapping chain - Unwrap follows the wrapper chain
	unwrapped1 := pkgerrors.Unwrap(fields)
	require.Equal(t, cause2, unwrapped1)

	unwrapped2 := pkgerrors.Unwrap(unwrapped1)
	require.Equal(t, level2Err, unwrapped2) // WithCause unwraps to wrapper

	unwrapped3 := pkgerrors.Unwrap(unwrapped2)
	require.Nil(t, unwrapped3) // level2Err is terminal

	// Test accessing causes via Cause() method
	causer, ok := cause2.(interface{ Cause() error })
	require.True(t, ok)
	require.Equal(t, cause1, causer.Cause())

	causer, ok = cause1.(interface{ Cause() error })
	require.True(t, ok)
	require.Equal(t, base, causer.Cause())
}

func TestConcurrentAccess(t *testing.T) {
	// Test that our error types are safe for concurrent access
	statusErr := errors.WithMetadata(errors.New("server error"), errors.StatusCode(500))
	fieldErr := errors.WithMetadata(statusErr, "concurrent", true)

	done := make(chan bool, 10)

	// Start multiple goroutines accessing the same error
	for i := range 10 {
		go func(id int) {
			defer func() { done <- true }()

			// Access error methods concurrently
			_ = fieldErr.Error()
			fields := errors.GetFields(fieldErr).List()
			require.Equal(t, []any{"concurrent", true}, fields)

			statusCode := errors.GetStatusCode(fieldErr)
			require.Equal(t, 500, statusCode)
		}(i)
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}
}

// WithMetadata Tests (Properties)

func TestWithMetadata_Properties(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("SingleProperty", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, errors.FaultInternal)
		require.NotNil(t, err)
		require.Equal(t, "base error", err.Error())
		require.Equal(t, errors.FaultInternal, errors.GetFault(err))
	})

	t.Run("MultipleProperties", func(t *testing.T) {
		err := errors.WithMetadata(baseErr,
			errors.FaultCaller,
			errors.StatusCode(400),
			errors.Retryable)
		require.NotNil(t, err)
		require.Equal(t, errors.FaultCaller, errors.GetFault(err))
		require.Equal(t, 400, errors.GetStatusCode(err))
		require.True(t, errors.IsRetryable(err))
	})

	t.Run("NilError", func(t *testing.T) {
		err := errors.WithMetadata(nil, errors.FaultInternal)
		require.Nil(t, err)
	})
}

func TestWithMetadata_Unwrap(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	propsErr := errors.WithMetadata(baseErr, errors.FaultInternal)

	t.Run("DirectUnwrap", func(t *testing.T) {
		unwrapped := stderrors.Unwrap(propsErr)
		require.Equal(t, baseErr, unwrapped)
	})

	t.Run("UnwrapThroughPkgErrors", func(t *testing.T) {
		unwrapped := pkgerrors.Unwrap(propsErr)
		require.Equal(t, baseErr, unwrapped)
	})

	t.Run("ErrorsIs", func(t *testing.T) {
		require.True(t, stderrors.Is(propsErr, baseErr))
		require.True(t, pkgerrors.Is(propsErr, baseErr))
	})

	t.Run("ErrorsAs", func(t *testing.T) {
		// Test that status codes are accessible through property wrappers
		statusErr := errors.WithMetadata(errors.New("not found"), errors.StatusCode(404))
		propsErr := errors.WithMetadata(statusErr, errors.FaultCaller)

		// Status code should be accessible through the chain
		require.Equal(t, 404, errors.GetStatusCode(propsErr))
		// Fault from outer layer should be accessible
		require.Equal(t, errors.FaultCaller, errors.GetFault(propsErr))
	})
}

func TestWithMetadata_PropertiesAndFields(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("SingleLayerBoth", func(t *testing.T) {
		// Now properties and fields can be set in a single call
		err := errors.WithMetadata(baseErr,
			errors.FaultInternal,
			errors.StatusCode(500),
			"key", "value")

		// All properties and fields accessible
		require.Equal(t, errors.FaultInternal, errors.GetFault(err))
		require.Equal(t, 500, errors.GetStatusCode(err))

		fields := errors.GetFields(err).List()
		require.Equal(t, []any{"key", "value"}, fields)

		// Should unwrap properly
		require.True(t, stderrors.Is(err, baseErr))
	})

	t.Run("MultipleLayers", func(t *testing.T) {
		// Can also chain multiple WithMetadata calls
		err1 := errors.WithMetadata(baseErr, "key1", "value1")
		err2 := errors.WithMetadata(err1, errors.FaultInternal, errors.StatusCode(500), "key2", "value2")

		// Properties accessible from outermost layer
		require.Equal(t, errors.FaultInternal, errors.GetFault(err2))
		require.Equal(t, 500, errors.GetStatusCode(err2))

		// Fields collected from all layers
		fields := errors.GetFields(err2).List()
		require.Equal(t, []any{"key2", "value2", "key1", "value1"}, fields)

		// Should unwrap through all layers
		require.True(t, stderrors.Is(err2, baseErr))
	})
}

func TestWithMetadata_MultipleLayersOfProperties(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	// Stack multiple property layers - each setting a different property
	err1 := errors.WithMetadata(baseErr, errors.FaultInternal)
	err2 := errors.WithMetadata(err1, errors.StatusCode(500))
	err3 := errors.WithMetadata(err2, errors.Retryable)

	// Getters now traverse the entire chain, returning the first non-zero value
	// This means properties set at any layer are accessible from the outermost error
	require.Equal(t, errors.FaultInternal, errors.GetFault(err3)) // Found in err1 (inner layer)
	require.Equal(t, 500, errors.GetStatusCode(err3))             // Found in err2 (middle layer)
	require.True(t, errors.IsRetryable(err3))                     // Found in err3 (outer layer)

	// Properties are still accessible when querying inner errors directly
	require.Equal(t, errors.FaultInternal, errors.GetFault(err1))
	require.Equal(t, 500, errors.GetStatusCode(err2))

	// Should unwrap through all layers
	require.True(t, stderrors.Is(err3, baseErr))
	require.True(t, stderrors.Is(err3, err1))
	require.True(t, stderrors.Is(err3, err2))
}

func TestWithMetadata_PropertyOverrides(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("OuterLayerOverridesInner", func(t *testing.T) {
		// Set fault at inner layer
		err1 := errors.WithMetadata(baseErr, errors.FaultInternal, errors.StatusCode(500))
		// Override fault at outer layer
		err2 := errors.WithMetadata(err1, errors.FaultCaller, "user_id", "123")

		// Outer layer's explicit value should win
		require.Equal(t, errors.FaultCaller, errors.GetFault(err2))
		// StatusCode not set in outer layer, so inner layer's value is used
		require.Equal(t, 500, errors.GetStatusCode(err2))

		// Fields from outer layer should be present
		fields := errors.GetFields(err2).List()
		require.Equal(t, []any{"user_id", "123"}, fields)
	})

	t.Run("InnerLayerUsedWhenOuterNotSet", func(t *testing.T) {
		// Set multiple properties at inner layer
		err1 := errors.WithMetadata(baseErr,
			errors.FaultInternal,
			errors.StatusCode(500),
			errors.Retryable)
		// Outer layer only adds fields, no properties
		err2 := errors.WithMetadata(err1, "key", "value")

		// All inner properties should be accessible
		require.Equal(t, errors.FaultInternal, errors.GetFault(err2))
		require.Equal(t, 500, errors.GetStatusCode(err2))
		require.True(t, errors.IsRetryable(err2))
	})

	t.Run("RetryabilityOverride", func(t *testing.T) {
		// Mark as retryable at inner layer
		err1 := errors.WithMetadata(baseErr, errors.Retryable)
		// Override to non-retryable at outer layer
		err2 := errors.WithMetadata(err1, errors.NonRetryable)

		// Outer layer should win
		require.False(t, errors.IsRetryable(err2))
	})
}

func TestWithMetadata_IntegrationWithBuilder(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("BuilderSetMethod", func(t *testing.T) {
		builder := errors.With(baseErr, "wrapped").
			Set(errors.FaultInternal, errors.StatusCode(500), "key", "value")

		// Extract the actual error from the builder
		err := builder.Err()

		// With the unified design, ALL properties and fields are accessible!
		require.Equal(t, errors.FaultInternal, errors.GetFault(err))
		require.Equal(t, 500, errors.GetStatusCode(err))

		// Check fields
		fields := errors.GetFields(err).List()
		require.Equal(t, []any{"key", "value"}, fields)

		// Check unwrapping works
		require.Contains(t, err.Error(), "wrapped")
		require.Contains(t, err.Error(), "base error")
	})

	t.Run("BuilderSetWithFields", func(t *testing.T) {
		errfs := errors.Fields{"user", "alice", "action", "login"}
		builder := errors.WithNew("auth failed").
			Set(errors.FaultCaller, errors.StatusCode(403), errfs)

		// Extract the actual error from the builder
		err := builder.Err()

		// Everything is accessible in a single layer!
		require.Equal(t, errors.FaultCaller, errors.GetFault(err))
		require.Equal(t, 403, errors.GetStatusCode(err))

		// Check fields
		fields := errors.GetFields(err).List()
		require.Equal(t, []any{"user", "alice", "action", "login"}, fields)
	})

	t.Run("BuilderMultipleSets", func(t *testing.T) {
		// Multiple Set() calls create layers, but getters traverse all layers
		builder := errors.WithNew("auth failed").
			Set(errors.FaultCaller, "user", "alice").
			Set(errors.StatusCode(403), "action", "login")

		err := builder.Err()

		// Getters now traverse the entire chain to find properties
		require.Equal(t, errors.FaultCaller, errors.GetFault(err)) // Found in inner layer
		require.Equal(t, 403, errors.GetStatusCode(err))           // Found in outer layer

		// All fields are collected from all layers
		fields := errors.GetFields(err).List()
		require.Equal(t, []any{"action", "login", "user", "alice"}, fields)
	})

	t.Run("BuilderAsError", func(t *testing.T) {
		// Test that Builder can be used directly as an error
		builder := errors.WithNew("test error").
			Set(errors.FaultInternal, errors.StatusCode(500))

		// Builder can be assigned to error interface
		var err error = builder
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "test error")

		// Extract the actual error for property inspection
		innerErr := builder.Err()
		require.Equal(t, errors.FaultInternal, errors.GetFault(innerErr))
		require.Equal(t, 500, errors.GetStatusCode(innerErr))
	})
}

func TestWithMetadata_CompatibilityWithStdlib(t *testing.T) {
	t.Run("WrapWithFmtErrorf", func(t *testing.T) {
		baseErr := pkgerrors.New("base")
		propsErr := errors.WithMetadata(baseErr, errors.FaultInternal)
		wrappedErr := fmt.Errorf("wrapped: %w", propsErr)

		// Should unwrap through fmt.Errorf wrapper
		require.True(t, stderrors.Is(wrappedErr, baseErr))

		// Properties should still be accessible
		require.Equal(t, errors.FaultInternal, errors.GetFault(wrappedErr))
	})

	t.Run("JoinedErrors", func(t *testing.T) {
		err1 := errors.WithMetadata(pkgerrors.New("error 1"), errors.FaultInternal)
		err2 := errors.WithMetadata(pkgerrors.New("error 2"), errors.FaultCaller)
		joined := stderrors.Join(err1, err2)

		require.NotNil(t, joined)

		// Should be able to find individual errors
		require.True(t, stderrors.Is(joined, err1))
		require.True(t, stderrors.Is(joined, err2))

		// Properties extraction should work on individual errors
		require.Equal(t, errors.FaultInternal, errors.GetFault(err1))
		require.Equal(t, errors.FaultCaller, errors.GetFault(err2))
	})
}

// Go 1.24 Standard Library Errors Package Compatibility Tests

func TestStdlibErrorsAs_WithCustomTypes(t *testing.T) {
	// Test status code extraction through various error wrappers
	statusErr := errors.WithMetadata(errors.New("internal error"), errors.StatusCode(500))
	fieldErr := errors.WithMetadata(statusErr, "component", "database")
	causeErr := errors.WithCause(statusErr, stderrors.New("connection failed"))
	wrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	t.Run("DirectStatusCode", func(t *testing.T) {
		require.Equal(t, 500, errors.GetStatusCode(statusErr))
	})

	t.Run("FieldWrappedStatusCode", func(t *testing.T) {
		require.Equal(t, 500, errors.GetStatusCode(fieldErr))
	})

	t.Run("CauseWrappedStatusCode", func(t *testing.T) {
		// WithCause properly maintains the error chain, so status code is accessible
		require.Equal(t, 500, errors.GetStatusCode(causeErr))
	})

	t.Run("StdWrappedStatusCode", func(t *testing.T) {
		require.Equal(t, 500, errors.GetStatusCode(wrappedErr))
	})

	t.Run("NonMatchingType", func(t *testing.T) {
		// Test with a built-in error type
		var pe *os.PathError
		result := stderrors.As(statusErr, &pe)
		require.False(t, result)
		require.Nil(t, pe)
	})
}

func TestStdlibErrorsIs_WithCustomTypes(t *testing.T) {
	// Test standard library errors.Is with our custom error types
	baseErr := errors.New("not found")
	statusErr := errors.WithMetadata(baseErr, errors.StatusCode(404))
	stdErr := stderrors.New("std error")

	fieldErr := errors.WithMetadata(statusErr, "resource", "user")
	causeErr := errors.WithCause(statusErr, stdErr)
	wrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	tests := []struct {
		name   string
		err    error
		target error
		expect bool
	}{
		{"DirectMatch", statusErr, statusErr, true},
		{"DirectMatchBase", statusErr, baseErr, true},
		{"FieldWrappedMatch", fieldErr, statusErr, true}, // withMetadata properly implements unwrapping
		{"CauseWrappedMatch", causeErr, statusErr, true}, // withCause now properly maintains chain
		{"StdWrappedMatch", wrappedErr, statusErr, true},
		{"NoMatch", statusErr, stdErr, false},
		{"CauseToStdErr", causeErr, stdErr, false}, // stdErr not in Unwrap chain (only accessible via Cause())
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stderrors.Is(tt.err, tt.target)
			require.Equal(t, tt.expect, result)
		})
	}
}

func TestStdlibErrorsUnwrap_WithCustomTypes(t *testing.T) {
	// Test standard library errors.Unwrap with our custom error types
	baseErr := errors.New("server error")
	statusErr := errors.WithMetadata(baseErr, errors.StatusCode(500))
	stdErr := stderrors.New("underlying error")

	fieldErr := errors.WithMetadata(statusErr, "key", "value")
	causeErr := errors.WithCause(statusErr, stdErr)
	stdWrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"WithMetadataUnwrap", statusErr, baseErr},
		{"FieldErrUnwrap", fieldErr, statusErr},
		{"CauseErrUnwrap", causeErr, statusErr}, // WithCause now unwraps to wrapper (statusErr)
		{"StdWrappedUnwrap", stdWrappedErr, statusErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stderrors.Unwrap(tt.err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestStdlibErrorsJoin_WithCustomTypes(t *testing.T) {
	// Test Go 1.20+ errors.Join with our custom error types
	statusErr := errors.WithMetadata(errors.New("bad request"), errors.StatusCode(400))
	stdErr := stderrors.New("validation failed")
	fieldErr := errors.WithMetadata(stderrors.New("database error"), "table", "users")

	// Join multiple error types
	joinedErr := stderrors.Join(statusErr, stdErr, fieldErr)
	require.NotNil(t, joinedErr)

	// Test that errors.Is works with joined errors
	require.True(t, stderrors.Is(joinedErr, statusErr))
	require.True(t, stderrors.Is(joinedErr, stdErr))
	require.True(t, stderrors.Is(joinedErr, fieldErr))

	// Test that GetStatusCode works with joined errors
	require.Equal(t, 400, errors.GetStatusCode(joinedErr))

	// Test error message contains all joined errors
	errMsg := joinedErr.Error()
	require.Contains(t, errMsg, "bad request")
	require.Contains(t, errMsg, "validation failed")
	require.Contains(t, errMsg, "database error")
}

func TestComplexStdlibErrorChains(t *testing.T) {
	// Test complex error chains mixing stdlib and custom errors
	baseStdErr := stderrors.New("connection timeout")
	statusErr := errors.WithMetadata(errors.New("service unavailable"), errors.StatusCode(503))

	// Chain: fmt.Errorf -> WithMetadata -> WithCause -> stdlib error
	level1 := fmt.Errorf("service failed: %w", statusErr)
	level2 := errors.WithMetadata(level1, "service", "auth", "retry_count", 3)
	level3 := errors.WithCause(level2, baseStdErr)

	// Test GetStatusCode works through the chain - WithCause now maintains the chain
	require.Equal(t, 503, errors.GetStatusCode(level3))

	// Test field extraction works - WithCause now maintains the field chain
	fields := errors.GetFields(level3).List()
	require.Equal(t, []any{"service", "auth", "retry_count", 3}, fields)

	// Test that Unwrap returns the wrapper (level2), not the cause
	require.Equal(t, level2, stderrors.Unwrap(level3))

	// Test that we can still access the cause via Cause() method
	causer, ok := level3.(interface{ Cause() error })
	require.True(t, ok)
	require.Equal(t, baseStdErr, causer.Cause())

	// Test error message composition
	errMsg := level3.Error()
	require.Contains(t, errMsg, "service unavailable")
	require.Contains(t, errMsg, "connection timeout")
}

func TestStdlibErrorCompatibility_EdgeCases(t *testing.T) {
	// Test edge cases for stdlib compatibility

	t.Run("NilErrorHandling", func(t *testing.T) {
		// Standard library should handle nil errors gracefully
		require.Nil(t, stderrors.Unwrap(nil))
		require.False(t, stderrors.Is(nil, stderrors.New("test")))
		require.Equal(t, 0, errors.GetStatusCode(nil))
	})

	t.Run("ErrorInterfaceImplementation", func(t *testing.T) {
		// Verify our custom types properly implement error interface
		statusErr := errors.WithMetadata(errors.New("ok"), errors.StatusCode(200))
		fieldErr := errors.WithMetadata(statusErr, "test", true)
		causeErr := errors.WithCause(statusErr, stderrors.New("cause"))

		// All should be assignable to error interface
		var err error
		err = statusErr
		require.NotNil(t, err)
		err = fieldErr
		require.NotNil(t, err)
		err = causeErr
		require.NotNil(t, err)
	})

	t.Run("UnwrapperInterfaceCompliance", func(t *testing.T) {
		// Test that our types implement the Unwrapper interface correctly
		baseErr := stderrors.New("base")
		fieldErr := errors.WithMetadata(baseErr, "key", "value")
		causeErr := errors.WithCause(stderrors.New("wrapper"), baseErr)

		// Test interface compliance
		type unwrapper interface {
			Unwrap() error
		}

		_, implementsUnwrap := fieldErr.(unwrapper)
		require.True(t, implementsUnwrap)

		_, implementsUnwrap = causeErr.(unwrapper)
		require.True(t, implementsUnwrap)
	})
}
