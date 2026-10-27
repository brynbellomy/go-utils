package errors

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
	require.EqualError(t, err, "base")
	require.True(t, Is(err, base))
}

func TestError_MethodsDoNotModifyReceiver(t *testing.T) {
	base := WithNew("base").Set(FaultCaller)

	wrapped := base.Wrap("context").Set(StatusCode(http.StatusNotFound), "id", 42).Stack().CausedBy(New("cause"))

	require.EqualError(t, base, "base")
	require.Equal(t, 0, GetStatusCode(base))
	require.Empty(t, GetFields(base).List())
	require.NotSame(t, base, wrapped)
	require.Equal(t, http.StatusNotFound, GetStatusCode(wrapped))
}

func TestError_SentinelsAreNotModifiedByWrapping(t *testing.T) {
	_ = With(ErrNotFound, "user 42").Set("id", 42)

	require.EqualError(t, ErrNotFound, "not found")
	require.Empty(t, GetFields(ErrNotFound).List())
}

func TestError_SentinelIdentity(t *testing.T) {
	viaFluent := With(ErrNotFound, "user 42").Set("id", 42)

	require.True(t, Is(viaFluent, ErrNotFound))
	require.True(t, Is(viaFluent.Err(), ErrNotFound))
	require.Equal(t, http.StatusNotFound, GetStatusCode(viaFluent))
}

func TestError_FormatMatchesHeldError(t *testing.T) {
	b := With(New("boom"), "context").Set("key", "value")

	for _, verb := range []string{"%s", "%v", "%+v", "%q"} {
		require.Equal(t, fmt.Sprintf(verb, b.Err()), fmt.Sprintf(verb, b), verb)
	}
	require.Contains(t, fmt.Sprintf("%+v", b), "error_test.go")
}

func TestError_FormatNil(t *testing.T) {
	var b *Error
	require.Equal(t, "<nil>", fmt.Sprintf("%v", b))
	require.Equal(t, "<nil>", fmt.Sprintf("%+v", b))
}

func TestError_CauseTraversesError(t *testing.T) {
	root := stderrors.New("root")
	b := With(root, "context").Set(FaultInternal)

	require.Same(t, root, Cause(b))
	require.True(t, OneOf(b, root))
	require.Nil(t, (*Error)(nil).Cause())
}

func TestSentinelRetryability(t *testing.T) {
	require.True(t, IsRetryable(ErrTooManyRequests))
	require.True(t, IsRetryable(ErrServiceUnavailable))
	require.False(t, IsRetryable(ErrNotFound))
}

func TestNew_AppliesItems(t *testing.T) {
	err := New("user missing", ErrNotFound, FaultUpstream, "id", 7, slog.String("tenant", "acme"))

	require.EqualError(t, err, "user missing")
	require.Equal(t, ErrNotFound, GetKind(err))
	require.Equal(t, FaultUpstream, GetFault(err))
	require.Equal(t, http.StatusNotFound, GetStatusCode(err))
	id, ok := LookupField(err, "id")
	require.True(t, ok)
	require.Equal(t, 7, id)
	tenant, ok := LookupField(err, "tenant")
	require.True(t, ok)
	require.Equal(t, "acme", tenant)
}

func TestWrap_NilReturnsUntypedNil(t *testing.T) {
	var typedNil *Error
	require.True(t, Wrap(nil, "x", ErrNotFound) == nil)
	require.True(t, Wrap(typedNil, "x") == nil)
	require.True(t, WithMetadata(nil, ErrNotFound) == nil)
}

func TestKind(t *testing.T) {
	t.Run("IsMatchesKindSetAsOption", func(t *testing.T) {
		err := Wrap(stderrors.New("no rows"), "load user", ErrNotFound)
		require.True(t, Is(err, ErrNotFound))
		require.False(t, Is(err, ErrBadRequest))
	})

	t.Run("IsMatchesWrappedKindValue", func(t *testing.T) {
		err := Wrap(ErrNotFound, "load user")
		require.True(t, Is(err, ErrNotFound))
		require.EqualError(t, err, "load user: not found")
		require.Equal(t, ErrNotFound, GetKind(err))
	})

	t.Run("DefaultsDeriveFromKind", func(t *testing.T) {
		err := Wrap(stderrors.New("busy"), "call", ErrServiceUnavailable)
		require.Equal(t, FaultInternal, GetFault(err))
		require.True(t, IsRetryable(err))
		require.Equal(t, http.StatusServiceUnavailable, GetStatusCode(err))
	})

	t.Run("ExplicitPropertyOverridesKindOnSameLayer", func(t *testing.T) {
		err := New("busy", ErrServiceUnavailable, NonRetryable, FaultUpstream, StatusCode(502))
		require.Equal(t, FaultUpstream, GetFault(err))
		require.Equal(t, NonRetryable, GetRetryability(err))
		require.Equal(t, 502, GetStatusCode(err))
	})

	t.Run("OuterKindOverridesInnerExplicitProperty", func(t *testing.T) {
		inner := New("inner", FaultInternal, StatusCode(500))
		outer := Wrap(inner, "outer", ErrBadRequest)
		require.Equal(t, FaultCaller, GetFault(outer))
		require.Equal(t, http.StatusBadRequest, GetStatusCode(outer))
		require.Equal(t, ErrBadRequest, GetKind(outer))
	})

	t.Run("StringsAreStable", func(t *testing.T) {
		require.Equal(t, "not_found", ErrNotFound.String())
		require.Equal(t, "not found", ErrNotFound.Error())
		require.Equal(t, "unknown", Kind(255).String())
		require.Equal(t, 0, KindUnknown.StatusCode())
	})
}

func TestGetFault_JoinTakesMostSevereChild(t *testing.T) {
	caller := New("a", FaultCaller)
	upstream := New("b", FaultUpstream)
	internal := New("c", FaultInternal)

	require.Equal(t, FaultUpstream, GetFault(Join(caller, upstream)))
	require.Equal(t, FaultInternal, GetFault(Join(upstream, internal, caller)))
	require.Equal(t, FaultCaller, GetFault(Join(stderrors.New("plain"), caller)))
	require.Equal(t, FaultUnknown, GetFault(Join(stderrors.New("plain"))))
}

func TestSentinel_HasNoStackUntilWrapped(t *testing.T) {
	sentinel := Sentinel("user missing", ErrNotFound)
	require.Empty(t, sentinel.(*Error).StackTrace())

	wrapped := Wrap(sentinel, "load")
	require.NotEmpty(t, wrapped.(*Error).StackTrace())
	require.True(t, Is(wrapped, sentinel))
	require.True(t, Is(wrapped, ErrNotFound))
}

type tenantID string

type describer interface{ Describe() string }

type region string

func (r region) Describe() string { return "region " + string(r) }

func TestProp_Lookup(t *testing.T) {
	inner := New("inner", Prop(tenantID("inner-tenant")), Prop(region("us-east")))
	outer := Wrap(inner, "outer", Prop(tenantID("outer-tenant")))

	id, ok := Lookup[tenantID](outer)
	require.True(t, ok)
	require.Equal(t, tenantID("outer-tenant"), id)

	d, ok := Lookup[describer](outer)
	require.True(t, ok)
	require.Equal(t, "region us-east", d.Describe())

	_, ok = Lookup[int](outer)
	require.False(t, ok)
	_, ok = Lookup[tenantID](nil)
	require.False(t, ok)
}

func TestErrorf_SupportsW(t *testing.T) {
	root := stderrors.New("root")
	err := Errorf("loading %s: %w", "config", root)

	require.EqualError(t, err, "loading config: root")
	require.True(t, Is(err, root))
	require.NotEmpty(t, err.(*Error).StackTrace())

	plain := Errorf("code %d", 7)
	require.EqualError(t, plain, "code 7")
	require.Nil(t, Unwrap(plain))
}

func TestWithCause_AsReachesCause(t *testing.T) {
	cause := &customErr{"cause"}
	err := WithCause(New("outer"), cause)

	got, ok := AsType[*customErr](err)
	require.True(t, ok)
	require.Same(t, cause, got)
	require.Same(t, error(cause), Cause(err))
}

type customErr struct{ msg string }

func (e *customErr) Error() string { return e.msg }

func TestStackTrace_OriginIsCallSite(t *testing.T) {
	err := Wrap(Wrap(stderrors.New("root"), "inner"), "outer")

	trace := err.(*Error).StackTrace()
	require.NotEmpty(t, trace)
	require.Contains(t, fmt.Sprintf("%+v", trace[0]), "TestStackTrace_OriginIsCallSite")
	require.Equal(t, 1, strings.Count(fmt.Sprintf("%+v", err), "runtime.goexit"))
}

func TestLogValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	err := Wrap(New("no rows", "table", "users"), "load user", ErrNotFound, "id", 7)
	logger.Error("request failed", "err", err)

	var out struct {
		Err map[string]any `json:"err"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &out))
	require.Equal(t, map[string]any{
		"msg":   "load user: no rows",
		"kind":  "not_found",
		"fault": "caller",
		"id":    float64(7),
		"table": "users",
	}, out.Err)
}

func TestOneOf_UsesIs(t *testing.T) {
	err := Wrap(fmt.Errorf("read: %w", io.EOF), "parse", ErrBadRequest)
	require.True(t, OneOf(err, io.ErrUnexpectedEOF, io.EOF))
	require.True(t, OneOf(err, ErrBadRequest))
	require.False(t, OneOf(err, io.ErrUnexpectedEOF))
	require.False(t, OneOf(nil, io.EOF))
}
