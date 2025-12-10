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

// StatusCoder Tests

func TestNewStatusCoder(t *testing.T) {
	tests := []struct {
		name    string
		code    int
		message string
	}{
		{"BadRequest", 400, "bad request"},
		{"NotFound", 404, "not found"},
		{"InternalServerError", 500, "internal server error"},
		{"CustomCode", 999, "custom error"},
		{"EmptyMessage", 200, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.NewStatusCoder(tt.code, tt.message)
			require.NotNil(t, err)
			require.Equal(t, tt.code, err.Code)
			require.Equal(t, tt.message, err.Message)
		})
	}
}

func TestStatusCoder_Error(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		message  string
		expected string
	}{
		{"Standard", 404, "not found", "404: not found"},
		{"WithSpaces", 400, "bad request", "400: bad request"},
		{"EmptyMessage", 200, "", "200: "},
		{"ZeroCode", 0, "zero error", "0: zero error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.NewStatusCoder(tt.code, tt.message)
			require.Equal(t, tt.expected, err.Error())
		})
	}
}

func TestAsStatusCoder(t *testing.T) {
	statusErr := errors.NewStatusCoder(404, "not found")
	regularErr := pkgerrors.New("regular error")
	wrappedStatusErr := pkgerrors.Wrap(statusErr, "wrapped")

	tests := []struct {
		name     string
		err      error
		expected *errors.StatusCoder
		ok       bool
	}{
		{"StatusCoder", statusErr, statusErr, true},
		{"RegularError", regularErr, nil, false},
		{"WrappedStatusCoder", wrappedStatusErr, statusErr, true},
		{"NilError", nil, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := errors.AsStatusCoder(tt.err)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsStatusCoder(t *testing.T) {
	statusErr404 := errors.NewStatusCoder(404, "not found")
	regularErr := pkgerrors.New("regular error")
	wrappedStatusErr := pkgerrors.Wrap(statusErr404, "wrapped")

	tests := []struct {
		name     string
		err      error
		codes    []int
		expected bool
	}{
		{"NoCodesStatusCoder", statusErr404, []int{}, true},
		{"NoCodesRegularError", regularErr, []int{}, false},
		{"SingleMatchingCode", statusErr404, []int{404}, true},
		{"SingleNonMatchingCode", statusErr404, []int{500}, false},
		{"MultipleCodesWithMatch", statusErr404, []int{400, 404, 500}, true},
		{"MultipleCodesNoMatch", statusErr404, []int{400, 500}, false},
		{"WrappedStatusCoder", wrappedStatusErr, []int{404}, true},
		{"NilError", nil, []int{404}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.IsStatusCoder(tt.err, tt.codes...)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPredefinedHTTPErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      *errors.StatusCoder
		code     int
		contains string
	}{
		{"BadRequest", errors.ErrBadRequest, http.StatusBadRequest, "bad"},
		{"Unauthorized", errors.ErrUnauthorized, http.StatusUnauthorized, "unauthorized"},
		{"Forbidden", errors.ErrForbidden, http.StatusForbidden, "forbidden"},
		{"NotFound", errors.ErrNotFound, http.StatusNotFound, "not found"},
		{"TooManyRequests", errors.ErrTooManyRequests, http.StatusTooManyRequests, "too many"},
		{"InternalServerError", errors.ErrInternalServerError, http.StatusInternalServerError, "internal"},
		{"Unimplemented", errors.ErrUnimplemented, http.StatusNotImplemented, "not implemented"},
		{"ServiceUnavailable", errors.ErrServiceUnavailable, http.StatusServiceUnavailable, "unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.code, tt.err.Code)
			require.Contains(t, tt.err.Message, tt.contains)
		})
	}
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

	// Test Cause() method
	causer, ok := result.(interface{ Cause() error })
	require.True(t, ok)
	require.Equal(t, cause, causer.Cause())

	// Test Unwrap() method
	require.Equal(t, cause, pkgerrors.Unwrap(result))
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

// WithFields Tests

func TestWithFields(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	fields := []any{"key1", "value1", "key2", 42}

	result := errors.WithFields(baseErr, fields...)
	require.NotNil(t, result)

	// Test Error() method
	require.Equal(t, "base error", result.Error())

	// Test Unwrap() method
	require.Equal(t, baseErr, pkgerrors.Unwrap(result))

	// Test Fields() extraction
	extractedFields := errors.Fields(result)
	require.Equal(t, fields, extractedFields)
}

func TestNewWithFields(t *testing.T) {
	fields := []any{"user_id", 123, "action", "login"}
	result := errors.NewWithFields("authentication failed", fields...)

	require.NotNil(t, result)
	require.Equal(t, "authentication failed", result.Error())

	extractedFields := errors.Fields(result)
	require.Equal(t, fields, extractedFields)
}

func TestWrapWithFields(t *testing.T) {
	originalErr := pkgerrors.New("database connection failed")
	fields := []any{"host", "localhost", "port", 5432}

	result := errors.WrapWithFields(originalErr, "failed to connect", fields...)
	require.NotNil(t, result)
	require.Equal(t, "failed to connect: database connection failed", result.Error())

	extractedFields := errors.Fields(result)
	require.Equal(t, fields, extractedFields)
}

func TestFields_MultipleNesting(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	fields1 := []any{"level", 1, "type", "database"}
	fields2 := []any{"level", 2, "operation", "query"}
	fields3 := []any{"level", 3, "table", "users"}

	// Create nested field errors
	err1 := errors.WithFields(baseErr, fields1...)
	err2 := errors.WithFields(err1, fields2...)
	err3 := errors.WithFields(err2, fields3...)

	// Fields should be collected from all levels
	allFields := errors.Fields(err3)
	expected := append(fields3, append(fields2, fields1...)...)
	require.Equal(t, expected, allFields)
}

func TestFields_NoFields(t *testing.T) {
	regularErr := pkgerrors.New("regular error")
	fields := errors.Fields(regularErr)
	require.Empty(t, fields)
}

func TestFields_EmptyFields(t *testing.T) {
	baseErr := pkgerrors.New("base error")
	result := errors.WithFields(baseErr)

	fields := errors.Fields(result)
	require.Empty(t, fields)
}

func TestWithFields_NilParent(t *testing.T) {
	fields := []any{"key", "value"}
	result := errors.WithFields(nil, fields...)

	require.Equal(t, "error with fields", result.Error())
	require.Nil(t, pkgerrors.Unwrap(result))

	extractedFields := errors.Fields(result)
	require.Equal(t, fields, extractedFields)
}

func TestWithFields_Format(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		format   string
		expected string
	}{
		{
			name:     "SimpleFields",
			err:      errors.NewWithFields("operation failed", "user_id", 123, "action", "login"),
			format:   "%s",
			expected: "operation failed user_id=123 action=login",
		},
		{
			name:     "StringWithSpaces",
			err:      errors.NewWithFields("request failed", "method", "GET", "path", "/api/v1/users"),
			format:   "%s",
			expected: "request failed method=GET path=/api/v1/users",
		},
		{
			name:     "StringRequiringQuotes",
			err:      errors.NewWithFields("error occurred", "message", "connection timed out", "code", 500),
			format:   "%s",
			expected: `error occurred message="connection timed out" code=500`,
		},
		{
			name:     "MixedTypes",
			err:      errors.NewWithFields("validation error", "field", "email", "value", "test@example.com", "required", true, "max_length", 255),
			format:   "%s",
			expected: "validation error field=email value=test@example.com required=true max_length=255",
		},
		{
			name:     "VerboseFormat",
			err:      errors.NewWithFields("base error", "key", "value"),
			format:   "%v",
			expected: "base error key=value",
		},
		{
			name:     "QuotedFormat",
			err:      errors.NewWithFields("error", "key", "value"),
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

func TestWithFields_FormatNested(t *testing.T) {
	baseErr := pkgerrors.New("database error")
	err1 := errors.WithFields(baseErr, "table", "users", "operation", "insert")
	err2 := errors.WithFields(err1, "user_id", 123, "retry", 3)

	result := fmt.Sprintf("%s", err2)
	// Fields should be collected from all levels
	require.Contains(t, result, "user_id=123")
	require.Contains(t, result, "retry=3")
	require.Contains(t, result, "table=users")
	require.Contains(t, result, "operation=insert")
	require.Contains(t, result, "database error")
}

func TestWithFields_FormatVerbosePlus(t *testing.T) {
	baseErr := pkgerrors.New("connection failed")
	fieldErr := errors.WithFields(baseErr, "host", "localhost", "port", 5432)

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
	// Test combining StatusCoder, WithCause, and WithFields
	baseErr := pkgerrors.New("database timeout")
	statusErr := errors.NewStatusCoder(500, "internal error")
	causeErr := errors.WithCause(statusErr, baseErr)
	fieldErr := errors.WithFields(causeErr, "user_id", 123, "operation", "read")

	// Test error message
	require.Contains(t, fieldErr.Error(), "internal error")
	require.Contains(t, fieldErr.Error(), "database timeout")

	// Test StatusCoder extraction directly from statusErr
	extracted, ok := errors.AsStatusCoder(statusErr)
	require.True(t, ok)
	require.Equal(t, 500, extracted.Code)

	// Test fields extraction
	fields := errors.Fields(fieldErr)
	require.Equal(t, []any{"user_id", 123, "operation", "read"}, fields)

	// Test cause unwrapping - fieldErr unwraps to causeErr, not baseErr
	require.Equal(t, causeErr, pkgerrors.Unwrap(fieldErr))
}

func TestStandardLibraryCompatibility(t *testing.T) {
	// Test compatibility with standard library pkgerrors.As and pkgerrors.Is
	statusErr := errors.NewStatusCoder(404, "not found")
	wrappedErr := pkgerrors.Wrap(statusErr, "wrapped")
	fieldErr := errors.WithFields(wrappedErr, "resource", "user")

	// Test pkgerrors.As with StatusCoder
	var sc *errors.StatusCoder
	require.True(t, pkgerrors.As(fieldErr, &sc))
	require.Equal(t, 404, sc.Code)

	// Test pkgerrors.Is behavior - since our errors use standard unwrapping,
	// this should actually work for direct comparisons
	require.True(t, pkgerrors.Is(wrappedErr, statusErr))
	// And also work for field-wrapped errors due to proper unwrapping chain
	require.True(t, pkgerrors.Is(fieldErr, statusErr))
}

func TestNilAndZeroValues(t *testing.T) {
	t.Run("NilStatusCoder", func(t *testing.T) {
		var sc *errors.StatusCoder
		// Test passing nil error to AsStatusCoder
		extracted, ok := errors.AsStatusCoder(nil)
		require.False(t, ok)
		require.Nil(t, extracted)
		_ = sc // Use sc to avoid unused variable
	})

	t.Run("ZeroValueStatusCoder", func(t *testing.T) {
		sc := &errors.StatusCoder{}
		require.Equal(t, "0: ", sc.Error())
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

		// Test that Unwrap() returns nil
		require.Nil(t, pkgerrors.Unwrap(result))
	})
}

func TestErrorUnwrappingChain(t *testing.T) {
	base := pkgerrors.New("base error")
	cause1 := errors.WithCause(pkgerrors.New("level 1"), base)
	cause2 := errors.WithCause(pkgerrors.New("level 2"), cause1)
	fields := errors.WithFields(cause2, "key", "value")

	// Test unwrapping chain
	unwrapped1 := pkgerrors.Unwrap(fields)
	require.Equal(t, cause2, unwrapped1)

	unwrapped2 := pkgerrors.Unwrap(unwrapped1)
	require.Equal(t, cause1, unwrapped2)

	unwrapped3 := pkgerrors.Unwrap(unwrapped2)
	require.Equal(t, base, unwrapped3)

	unwrapped4 := pkgerrors.Unwrap(unwrapped3)
	require.Nil(t, unwrapped4)
}

func TestConcurrentAccess(t *testing.T) {
	// Test that our error types are safe for concurrent access
	statusErr := errors.NewStatusCoder(500, "server error")
	fieldErr := errors.WithFields(statusErr, "concurrent", true)

	done := make(chan bool, 10)

	// Start multiple goroutines accessing the same error
	for i := range 10 {
		go func(id int) {
			defer func() { done <- true }()

			// Access error methods concurrently
			_ = fieldErr.Error()
			fields := errors.Fields(fieldErr)
			require.Equal(t, []any{"concurrent", true}, fields)

			extracted, ok := errors.AsStatusCoder(fieldErr)
			require.True(t, ok)
			require.Equal(t, 500, extracted.Code)
		}(i)
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}
}

// Go 1.24 Standard Library Errors Package Compatibility Tests

func TestStdlibErrorsAs_WithCustomTypes(t *testing.T) {
	// Test standard library errors.As with our custom error types
	statusErr := errors.NewStatusCoder(500, "internal error")
	fieldErr := errors.WithFields(statusErr, "component", "database")
	causeErr := errors.WithCause(statusErr, stderrors.New("connection failed"))
	wrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	t.Run("DirectStatusCoder", func(t *testing.T) {
		var sc *errors.StatusCoder
		result := stderrors.As(statusErr, &sc)
		require.True(t, result)
		require.Equal(t, 500, sc.Code)
	})

	t.Run("FieldWrappedStatusCoder", func(t *testing.T) {
		var sc *errors.StatusCoder
		result := stderrors.As(fieldErr, &sc)
		require.True(t, result)
		require.Equal(t, 500, sc.Code)
	})

	t.Run("CauseWrappedStatusCoder", func(t *testing.T) {
		var sc *errors.StatusCoder
		result := stderrors.As(causeErr, &sc)
		// WithCause wraps errors in a way that doesn't expose the original StatusCoder to errors.As
		require.False(t, result)
		require.Nil(t, sc)
	})

	t.Run("StdWrappedStatusCoder", func(t *testing.T) {
		var sc *errors.StatusCoder
		result := stderrors.As(wrappedErr, &sc)
		require.True(t, result)
		require.Equal(t, 500, sc.Code)
	})

	t.Run("NonMatchingType", func(t *testing.T) {
		// Test with a built-in error type that doesn't match
		var pe *os.PathError
		result := stderrors.As(statusErr, &pe)
		require.False(t, result)
		require.Nil(t, pe)
	})
}

func TestStdlibErrorsIs_WithCustomTypes(t *testing.T) {
	// Test standard library errors.Is with our custom error types
	statusErr := errors.NewStatusCoder(404, "not found")
	stdErr := stderrors.New("std error")

	fieldErr := errors.WithFields(statusErr, "resource", "user")
	causeErr := errors.WithCause(statusErr, stdErr)
	wrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	tests := []struct {
		name   string
		err    error
		target error
		expect bool
	}{
		{"DirectMatch", statusErr, statusErr, true},
		{"FieldWrappedMatch", fieldErr, statusErr, true},  // withFields properly implements unwrapping
		{"CauseWrappedMatch", causeErr, statusErr, false}, // withCause has custom unwrapping
		{"StdWrappedMatch", wrappedErr, statusErr, true},
		{"NoMatch", statusErr, stdErr, false},
		{"CauseToStdErr", causeErr, stdErr, true}, // Should find std error through cause
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
	statusErr := errors.NewStatusCoder(500, "server error")
	stdErr := stderrors.New("underlying error")

	fieldErr := errors.WithFields(statusErr, "key", "value")
	causeErr := errors.WithCause(statusErr, stdErr)
	stdWrappedErr := fmt.Errorf("wrapped: %w", statusErr)

	tests := []struct {
		name     string
		err      error
		expected error
	}{
		{"StatusCoderNoUnwrap", statusErr, nil},
		{"FieldErrUnwrap", fieldErr, statusErr},
		{"CauseErrUnwrap", causeErr, stdErr}, // WithCause unwraps to cause, not wrapped error
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
	statusErr := errors.NewStatusCoder(400, "bad request")
	stdErr := stderrors.New("validation failed")
	fieldErr := errors.WithFields(stderrors.New("database error"), "table", "users")

	// Join multiple error types
	joinedErr := stderrors.Join(statusErr, stdErr, fieldErr)
	require.NotNil(t, joinedErr)

	// Test that errors.Is works with joined errors
	require.True(t, stderrors.Is(joinedErr, statusErr))
	require.True(t, stderrors.Is(joinedErr, stdErr))
	require.True(t, stderrors.Is(joinedErr, fieldErr))

	// Test that errors.As works with joined errors
	var sc *errors.StatusCoder
	require.True(t, stderrors.As(joinedErr, &sc))
	require.Equal(t, 400, sc.Code)

	// Test error message contains all joined errors
	errMsg := joinedErr.Error()
	require.Contains(t, errMsg, "bad request")
	require.Contains(t, errMsg, "validation failed")
	require.Contains(t, errMsg, "database error")
}

func TestComplexStdlibErrorChains(t *testing.T) {
	// Test complex error chains mixing stdlib and custom errors
	baseStdErr := stderrors.New("connection timeout")
	statusErr := errors.NewStatusCoder(503, "service unavailable")

	// Chain: fmt.Errorf -> WithFields -> WithCause -> stdlib error
	level1 := fmt.Errorf("service failed: %w", statusErr)
	level2 := errors.WithFields(level1, "service", "auth", "retry_count", 3)
	level3 := errors.WithCause(level2, baseStdErr)

	// Test errors.As works through the chain - but WithCause breaks the chain
	var sc *errors.StatusCoder
	result := stderrors.As(level3, &sc)
	// The WithCause at level3 prevents errors.As from finding the StatusCoder
	require.False(t, result)

	// Test field extraction works - but WithCause breaks the field chain too
	fields := errors.Fields(level3)
	require.Empty(t, fields) // WithCause prevents field extraction from level2

	// Test that we can find the base error through cause unwrapping
	require.Equal(t, baseStdErr, stderrors.Unwrap(level3))

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
		var sc *errors.StatusCoder
		require.False(t, stderrors.As(nil, &sc))
	})

	t.Run("ErrorInterfaceImplementation", func(t *testing.T) {
		// Verify our custom types properly implement error interface
		statusErr := errors.NewStatusCoder(200, "ok")
		fieldErr := errors.WithFields(statusErr, "test", true)
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
		fieldErr := errors.WithFields(baseErr, "key", "value")
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
