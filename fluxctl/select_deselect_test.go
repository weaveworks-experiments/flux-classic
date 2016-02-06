package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

func TestSelect(t *testing.T) {
	// No such service
	_, err := runOpts(&selectOpts{}, []string{
		"doop-svc", "bar-rule", "--image", "whatever",
	})
	require.Error(t, err)

	// For the ff we need a service to attach rules to
	st, err := runOpts(&addOpts{}, []string{
		"foo-svc",
	})
	require.NoError(t, err)

	// Don't select anything
	err = runOptsWithStore(&selectOpts{}, st, []string{
		"foo-svc", "empty-rule",
	})
	require.Error(t, err)

	opts := &selectOpts{}
	bout, berr := opts.tapOutput()
	err = runOptsWithStore(opts, st, []string{
		"foo-svc", "ok-rule", "--image", "foo/bar",
	})
	require.NoError(t, err)
	require.Equal(t, "ok-rule\n", bout.String())
	require.Equal(t, "", berr.String())

	svc, err := st.GetService("foo-svc", store.QueryServiceOptions{WithContainerRules: true})
	require.NoError(t, err)
	require.Len(t, svc.ContainerRules, 1)
	rule := svc.ContainerRules[0]
	require.Equal(t, store.ContainerRuleInfo{
		Name: "ok-rule",
		ContainerRule: data.ContainerRule{
			Selector: map[string]string{
				"image": "foo/bar",
			},
		},
	}, rule)
}
