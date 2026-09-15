package errors_test

import (
	stderrors "errors"
	"testing"

	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/brynbellomy/go-utils/errors"
)

func TestFaultString(t *testing.T) {
	tests := []struct {
		name  string
		fault errors.Fault
		want  string
	}{
		{"Unknown", errors.FaultUnknown, "unknown"},
		{"Caller", errors.FaultCaller, "caller"},
		{"Internal", errors.FaultInternal, "internal"},
		{"Upstream", errors.FaultUpstream, "upstream"},
		{"OutOfRange", errors.Fault(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.fault.String())
		})
	}
}

func TestRetryabilityString(t *testing.T) {
	tests := []struct {
		name string
		r    errors.Retryability
		want string
	}{
		{"Unknown", errors.UnknownRetryability, "unknown"},
		{"Retryable", errors.Retryable, "retryable"},
		{"NonRetryable", errors.NonRetryable, "non_retryable"},
		{"OutOfRange", errors.Retryability(255), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.r.String())
		})
	}
}

func TestGetRetryability(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("Unset", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, "key", "value")
		require.Equal(t, errors.UnknownRetryability, errors.GetRetryability(err))
		require.False(t, errors.IsRetryable(err))
	})

	t.Run("InnerRetryable", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.Retryable)
		outer := errors.WithMetadata(inner, "key", "value")
		require.Equal(t, errors.Retryable, errors.GetRetryability(outer))
		require.True(t, errors.IsRetryable(outer))
	})

	t.Run("OuterNonRetryableOverridesInnerRetryable", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.Retryable)
		outer := errors.WithMetadata(inner, errors.NonRetryable)
		require.Equal(t, errors.NonRetryable, errors.GetRetryability(outer))
		require.False(t, errors.IsRetryable(outer))
	})

	t.Run("TypedNil", func(t *testing.T) {
		var typedNil *pointerError
		var err error = typedNil
		require.Equal(t, errors.UnknownRetryability, errors.GetRetryability(err))
		require.False(t, errors.IsRetryable(err))
	})

	t.Run("ThroughPkgErrorsWrap", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.Retryable)
		wrapped := pkgerrors.Wrap(inner, "wrapped")
		require.Equal(t, errors.Retryable, errors.GetRetryability(wrapped))
		require.True(t, errors.IsRetryable(wrapped))
	})

	t.Run("ThroughWithCause", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.Retryable)
		causeErr := errors.WithCause(inner, pkgerrors.New("cause"))
		require.Equal(t, errors.Retryable, errors.GetRetryability(causeErr))
		require.True(t, errors.IsRetryable(causeErr))
	})

	t.Run("Nil", func(t *testing.T) {
		require.Equal(t, errors.UnknownRetryability, errors.GetRetryability(nil))
		require.False(t, errors.IsRetryable(nil))
	})
}

func TestFaultUpstream(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("RoundTripsThroughGetFault", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, errors.FaultUpstream)
		require.Equal(t, errors.FaultUpstream, errors.GetFault(err))
	})

	t.Run("RoundTripsThroughBuilderSet", func(t *testing.T) {
		builder := errors.With(baseErr, "rpc call failed").Set(errors.FaultUpstream)
		err := builder.Err()
		require.Equal(t, errors.FaultUpstream, errors.GetFault(err))
	})

	t.Run("StringMatchesLabel", func(t *testing.T) {
		require.Equal(t, "upstream", errors.FaultUpstream.String())
	})
}

func TestLookupField(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("Absent", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, "foo", "bar")
		val, ok := errors.LookupField(err, "missing")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("Present", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, "foo", "bar")
		val, ok := errors.LookupField(err, "foo")
		require.True(t, ok)
		require.Equal(t, "bar", val)
	})

	t.Run("OuterOverridesInner", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, "key", "inner-value")
		outer := errors.WithMetadata(inner, "key", "outer-value")
		val, ok := errors.LookupField(outer, "key")
		require.True(t, ok)
		require.Equal(t, "outer-value", val)
	})

	t.Run("NonStringKeyIgnored", func(t *testing.T) {
		err := errors.WithMetadata(baseErr, 42, "should-be-ignored", "real-key", "real-value")
		val, ok := errors.LookupField(err, "real-key")
		require.True(t, ok)
		require.Equal(t, "real-value", val)
	})

	t.Run("DanglingKeyDoesNotPanic", func(t *testing.T) {
		fields := errors.Fields{"a"}
		require.NotPanics(t, func() {
			val, ok := fields.Lookup("a")
			require.False(t, ok)
			require.Nil(t, val)
		})
	})

	t.Run("NilErr", func(t *testing.T) {
		val, ok := errors.LookupField(nil, "foo")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("TypedNilErr", func(t *testing.T) {
		var typedNil *pointerError
		var err error = typedNil
		val, ok := errors.LookupField(err, "foo")
		require.False(t, ok)
		require.Nil(t, val)
	})
}

func TestFieldsLookup(t *testing.T) {
	t.Run("Found", func(t *testing.T) {
		fields := errors.Fields{"a", 1, "b", 2}
		val, ok := fields.Lookup("b")
		require.True(t, ok)
		require.Equal(t, 2, val)
	})

	t.Run("NotFound", func(t *testing.T) {
		fields := errors.Fields{"a", 1}
		val, ok := fields.Lookup("z")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("EmptyFields", func(t *testing.T) {
		var fields errors.Fields
		val, ok := fields.Lookup("a")
		require.False(t, ok)
		require.Nil(t, val)
	})
}

func TestGetFault_JoinedErrors(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("SingleJoinedClassifiedError", func(t *testing.T) {
		classified := errors.WithMetadata(baseErr, errors.FaultInternal)
		joined := errors.Join(classified)
		require.Equal(t, errors.FaultInternal, errors.GetFault(joined))
	})

	t.Run("FoundInSecondChild", func(t *testing.T) {
		unclassified := pkgerrors.New("plain error")
		classified := errors.WithMetadata(baseErr, errors.FaultCaller)
		joined := errors.Join(unclassified, classified)
		require.Equal(t, errors.FaultCaller, errors.GetFault(joined))
	})

	t.Run("OuterMetadataOverridesJoinedChild", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.FaultCaller)
		joined := errors.Join(inner)
		outer := errors.WithMetadata(joined, errors.FaultInternal)
		require.Equal(t, errors.FaultInternal, errors.GetFault(outer))
	})

	t.Run("JoinNestedInsideWrapInsideJoin", func(t *testing.T) {
		innermost := errors.WithMetadata(baseErr, errors.FaultUpstream)
		innerJoin := errors.Join(innermost)
		wrapped := pkgerrors.Wrap(innerJoin, "wrapped")
		outerJoin := errors.Join(wrapped)
		require.Equal(t, errors.FaultUpstream, errors.GetFault(outerJoin))
	})

	t.Run("NoClassifiedChild", func(t *testing.T) {
		joined := errors.Join(pkgerrors.New("a"), pkgerrors.New("b"))
		require.Equal(t, errors.FaultUnknown, errors.GetFault(joined))
	})
}

func TestGetRetryability_JoinedErrors(t *testing.T) {
	baseErr := pkgerrors.New("base error")

	t.Run("SingleJoinedClassifiedError", func(t *testing.T) {
		classified := errors.WithMetadata(baseErr, errors.NonRetryable)
		joined := errors.Join(classified)
		require.Equal(t, errors.NonRetryable, errors.GetRetryability(joined))
		require.False(t, errors.IsRetryable(joined))
	})

	t.Run("FoundInSecondChild", func(t *testing.T) {
		unclassified := pkgerrors.New("plain error")
		classified := errors.WithMetadata(baseErr, errors.Retryable)
		joined := errors.Join(unclassified, classified)
		require.Equal(t, errors.Retryable, errors.GetRetryability(joined))
		require.True(t, errors.IsRetryable(joined))
	})

	t.Run("OuterMetadataOverridesJoinedChild", func(t *testing.T) {
		inner := errors.WithMetadata(baseErr, errors.Retryable)
		joined := errors.Join(inner)
		outer := errors.WithMetadata(joined, errors.NonRetryable)
		require.Equal(t, errors.NonRetryable, errors.GetRetryability(outer))
		require.False(t, errors.IsRetryable(outer))
	})

	t.Run("JoinNestedInsideWrapInsideJoin", func(t *testing.T) {
		innermost := errors.WithMetadata(baseErr, errors.Retryable)
		innerJoin := errors.Join(innermost)
		wrapped := pkgerrors.Wrap(innerJoin, "wrapped")
		outerJoin := errors.Join(wrapped)
		require.Equal(t, errors.Retryable, errors.GetRetryability(outerJoin))
		require.True(t, errors.IsRetryable(outerJoin))
	})

	t.Run("NoClassifiedChild", func(t *testing.T) {
		joined := errors.Join(pkgerrors.New("a"), pkgerrors.New("b"))
		require.Equal(t, errors.UnknownRetryability, errors.GetRetryability(joined))
		require.False(t, errors.IsRetryable(joined))
	})
}

func TestGetFields_JoinedErrors(t *testing.T) {
	t.Run("BothChildrenContributeInOrder", func(t *testing.T) {
		a := errors.WithMetadata(pkgerrors.New("error a"), "k", 1)
		b := errors.WithMetadata(pkgerrors.New("error b"), "k", 2)
		joined := errors.Join(a, b)

		fields := errors.GetFields(joined).List()
		require.Equal(t, []any{"k", 1, "k", 2}, fields)

		val, ok := errors.LookupField(joined, "k")
		require.True(t, ok)
		require.Equal(t, 1, val)
	})

	t.Run("EachChildWalkedOuterToInner", func(t *testing.T) {
		aInner := errors.WithMetadata(pkgerrors.New("a inner"), "level", "inner-a")
		aOuter := errors.WithMetadata(aInner, "level", "outer-a")
		bInner := errors.WithMetadata(pkgerrors.New("b inner"), "level", "inner-b")
		bOuter := errors.WithMetadata(bInner, "level", "outer-b")
		joined := errors.Join(aOuter, bOuter)

		fields := errors.GetFields(joined).List()
		require.Equal(t, []any{
			"level", "outer-a", "level", "inner-a",
			"level", "outer-b", "level", "inner-b",
		}, fields)

		val, ok := errors.LookupField(joined, "level")
		require.True(t, ok)
		require.Equal(t, "outer-a", val)
	})

	t.Run("JoinNestedInsideWrapInsideJoin", func(t *testing.T) {
		innermost := errors.WithMetadata(pkgerrors.New("base"), "key", "value")
		innerJoin := errors.Join(innermost)
		wrapped := pkgerrors.Wrap(innerJoin, "wrapped")
		outerJoin := errors.Join(wrapped)

		fields := errors.GetFields(outerJoin).List()
		require.Equal(t, []any{"key", "value"}, fields)
	})

	t.Run("NoFieldsInAnyChild", func(t *testing.T) {
		joined := errors.Join(pkgerrors.New("a"), pkgerrors.New("b"))
		require.Empty(t, errors.GetFields(joined).List())
	})
}

func TestIsAsReExports(t *testing.T) {
	sentinel := stderrors.New("sentinel error")
	wrapped := pkgerrors.Wrap(sentinel, "wrapped")

	t.Run("Is", func(t *testing.T) {
		require.True(t, errors.Is(wrapped, sentinel))
		require.False(t, errors.Is(wrapped, stderrors.New("different")))
	})

	t.Run("As", func(t *testing.T) {
		pe := &pointerError{msg: "pointer error"}
		wrappedPe := pkgerrors.Wrap(pe, "wrapped pointer error")

		var target *pointerError
		require.True(t, errors.As(wrappedPe, &target))
		require.Equal(t, pe, target)
	})
}
