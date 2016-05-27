package agent

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
)

func TestTee(t *testing.T) {
	in := make(chan int)
	out1 := make(chan int)
	out2 := make(chan int)

	errs := daemon.NewErrorSink()

	tee := Tee(in, out1, out2)(errs)

	in <- 42
	require.Equal(t, 42, <-out1)
	require.Equal(t, 42, <-out2)
	in <- 54321
	require.Equal(t, 54321, <-out1)
	require.Equal(t, 54321, <-out2)

	tee.Stop()
	require.Empty(t, errs)
}
