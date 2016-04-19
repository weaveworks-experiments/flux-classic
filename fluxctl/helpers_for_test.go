package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

func (cmd *baseOpts) tapOutput() (*bytes.Buffer, *bytes.Buffer) {
	bufout := new(bytes.Buffer)
	buferr := new(bytes.Buffer)
	cmd.redirect(bufout, buferr)
	return bufout, buferr
}

type testOpts struct {
	baseOpts
}

func TestTapOutput(t *testing.T) {
	opts := &testOpts{}
	bout, berr := opts.tapOutput()
	opts.getStdout().Write([]byte("this goes to out"))
	opts.getStderr().Write([]byte("this goes to err"))
	require.Equal(t, "this goes to out", bout.String())
	require.Equal(t, "this goes to err", berr.String())
}

func runOpts(opts commandOpts, args []string) (store.Store, error) {
	st := inmem.NewInMem().Store("test fluxctl")
	return st, runOptsWithStore(opts, st, args)
}

func runOptsWithStore(opts commandOpts, store store.Store, args []string) error {
	opts.setStore(store)
	cmd := opts.makeCommand()
	_ = cmd.Flags().Bool("test-dummy", false, "should not be seen")
	cmd.Flags().MarkHidden("test-dummy")
	cmd.SetArgs(append(args, "--test-dummy"))
	return cmd.Execute()
}
