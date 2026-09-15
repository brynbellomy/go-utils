package errors

import (
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
