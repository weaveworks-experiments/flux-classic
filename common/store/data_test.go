package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func toMap(vals []string) map[string]string {
	if len(vals)%2 != 0 {
		panic("Need an even number of arguments, label value ...")
	}
	m := make(map[string]string)
	for i := 0; i < len(vals); i += 2 {
		m[vals[i]] = vals[i+1]
	}
	return m
}

func makeSpec(kv ...string) ContainerRule {
	return ContainerRule{
		Selector: Selector(toMap(kv)),
	}
}

type lbld map[string]string

func (inst lbld) Label(k string) string {
	return inst[k]
}

func inst(kv ...string) Labeled {
	return lbld(toMap(kv))
}

func TestIncludes(t *testing.T) {
	assert := assert.New(t)
	spec := makeSpec("foo", "bar")
	assert.True(spec.Includes(inst("foo", "bar")))
	assert.True(spec.Includes(
		inst(
			"bar", "whatever",
			"foo", "bar")))
	assert.False(spec.Includes(
		inst("foo", "nope")))
}
