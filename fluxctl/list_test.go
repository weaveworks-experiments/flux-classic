package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store/inmem"
)

func TestList(t *testing.T) {
	st := inmem.NewInMemStore()
	err := st.AddService("foo", data.Service{})
	require.NoError(t, err)

	opts := &listOpts{}
	bufout, buferr := opts.tapOutput()
	err = runOptsWithStore(opts, st, []string{})
	require.NoError(t, err)
	require.Equal(t, "foo\n", bufout.String())
	require.Equal(t, "", buferr.String())
}
