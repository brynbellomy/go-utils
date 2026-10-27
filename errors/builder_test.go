package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithNew_FormatsStringWithArgs(t *testing.T) {
	err := WithNew("invalid height %d for %s", 42, "block").Err()
	require.EqualError(t, err, "invalid height 42 for block")
}

func TestWithNew_PlainStringUnchanged(t *testing.T) {
	err := WithNew("100% literal").Err()
	require.EqualError(t, err, "100% literal")
}

func TestWithNew_ErrorIgnoresArgs(t *testing.T) {
	base := New("base")
	err := WithNew(base, "ignored", 1).Err()
	require.Same(t, base, err)
}

func TestBuilder_MethodsDoNotModifyReceiver(t *testing.T) {
	base := WithNew("base").Set(FaultCaller)

	wrapped := base.Wrap("context").Set(StatusCode(http.StatusNotFound), "id", 42).Stack().CausedBy(New("cause"))

	require.EqualError(t, base, "base")
	require.Equal(t, 0, GetStatusCode(base))
	require.Empty(t, GetFields(base).List())
	require.NotSame(t, base, wrapped)
	require.Equal(t, http.StatusNotFound, GetStatusCode(wrapped))
}

func TestBuilder_SentinelsAreNotModifiedByWrapping(t *testing.T) {
	_ = With(ErrNotFound, "user 42").Set("id", 42)

	require.EqualError(t, ErrNotFound, "not found")
	require.Empty(t, GetFields(ErrNotFound).List())
}

func TestBuilder_SentinelIdentity(t *testing.T) {
	viaBuilder := With(ErrNotFound, "user 42").Set("id", 42)

	require.True(t, Is(viaBuilder, ErrNotFound))
	require.True(t, Is(viaBuilder.Err(), ErrNotFound))
	require.Equal(t, http.StatusNotFound, GetStatusCode(viaBuilder))
}

func TestBuilder_FormatMatchesHeldError(t *testing.T) {
	b := With(New("boom"), "context").Set("key", "value")

	for _, verb := range []string{"%s", "%v", "%+v", "%q"} {
		require.Equal(t, fmt.Sprintf(verb, b.Err()), fmt.Sprintf(verb, b), verb)
	}
	require.Contains(t, fmt.Sprintf("%+v", b), "builder_test.go")
}

func TestBuilder_FormatNil(t *testing.T) {
	var b *Builder
	require.Equal(t, "<nil>", fmt.Sprintf("%v", b))
	require.Equal(t, "<nil>", fmt.Sprintf("%+v", b))
}

func TestBuilder_CauseTraversesBuilder(t *testing.T) {
	root := stderrors.New("root")
	b := With(root, "context").Set(FaultInternal)

	require.Same(t, root, Cause(b))
	require.True(t, OneOf(b, root))
	require.Nil(t, (*Builder)(nil).Cause())
}

func TestSentinelRetryability(t *testing.T) {
	require.True(t, IsRetryable(ErrTooManyRequests))
	require.True(t, IsRetryable(ErrServiceUnavailable))
	require.False(t, IsRetryable(ErrNotFound))
}
