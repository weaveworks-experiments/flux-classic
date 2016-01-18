package main

import (
	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/inmem"
)

func runOpts(opts commandOpts, args []string) (store.Store, error) {
	st := inmem.NewInMemStore()
	return st, runOptsWithStore(opts, st, args)
}

func runOptsWithStore(opts commandOpts, store store.Store, args []string) error {
	opts.setStore(store)
	cmd := opts.makeCommand()
	cmd.SetArgs(args)
	return cmd.Execute()
}
