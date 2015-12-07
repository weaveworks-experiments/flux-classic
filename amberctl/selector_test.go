package main

import (
	"testing"

	"github.com/squaremo/ambergreen/common/data"

	"github.com/stretchr/testify/require"
)

func sel(kv []string) data.Selector {
	m := make(map[string]string)
	if len(kv)%2 != 0 {
		panic("Expected sel(k, v, ...)")
	}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return data.Selector(m)
}

func testSel(t *testing.T, s selector, kv ...string) {
	require.Equal(t, sel(kv), (&s).makeSelector())
}

func TestMakeSelector(t *testing.T) {
	testSel(t, selector{})
	testSel(t, selector{image: "foobar/baz"}, "image", "foobar/baz")
	testSel(t, selector{tag: "latest"}, "tag", "latest")
	testSel(t, selector{labels: "foo=bar"}, "foo", "bar")
	testSel(t, selector{env: "foo=bar"}, "env.foo", "bar")
	testSel(t, selector{labels: "foo=bar,foop=barp"}, "foo", "bar", "foop", "barp")
	testSel(t, selector{env: "foo=bar,foop=barp"}, "env.foo", "bar", "env.foop", "barp")
	// leading commas, duplicated commas
	testSel(t, selector{labels: ",,foop=barp,"}, "foop", "barp")
	// override label with specific image
	testSel(t, selector{image: "foo", labels: "image=bar"}, "image", "foo")
	// equals in value
	testSel(t, selector{env: "SERVICE=pages=>borp"}, "env.SERVICE", "pages=>borp")
	// leading space in key is excluded; leading and trailing space in value is included
	testSel(t, selector{labels: ", foo= bar ,"}, "foo", " bar ")
}
