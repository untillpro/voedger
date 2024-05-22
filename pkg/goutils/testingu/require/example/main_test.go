package main

import (
	"errors"
	"testing"

	"github.com/voedger/voedger/pkg/goutils/testingu/require"
)

func TestGoCrazy(t *testing.T) {
	require := require.New(t)

	require.PanicsWith(
		GoCrazy,
		require.Is(CrazyError, "panic error should be %v", CrazyError),
		require.Is(errors.ErrUnsupported),
		require.Has("🤪", "panic should contains crazy smile %q", "🤪"),
		require.Has("unsupported"),
	)
}

func TestCrazyError(t *testing.T) {
	require := require.New(t)

	require.ErrorWith(
		CrazyError,
		require.Is(errors.ErrUnsupported),
		require.Has("🤪"),
	)
}
