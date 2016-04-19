package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

func TestList(t *testing.T) {
	st := inmem.NewInMem().Store("test fluxctl list")
	err := st.AddService("foo", store.Service{})
	require.NoError(t, err)

	opts := &listOpts{}
	bufout, buferr := opts.tapOutput()
	err = runOptsWithStore(opts, st, []string{})
	require.NoError(t, err)
	require.Equal(t, "foo\n", bufout.String())
	require.Equal(t, "", buferr.String())
}
