package main

import (
	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/inmem"
)

func runOpts(opts commandOpts, args []string) (store.Store, error) {
	st := inmem.NewInMemStore()
	opts.setStore(st)
	cmd := opts.makeCommand()
	cmd.SetArgs(args)
	err := cmd.Execute()
	return st, err
}
