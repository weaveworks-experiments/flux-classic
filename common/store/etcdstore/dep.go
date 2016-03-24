package etcdstore

import (
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/store"
)

type dependencySlot struct {
	slot *store.Store
}

type dependencyKey struct{}

func StoreDependency(slot *store.Store) daemon.DependencySlot {
	return dependencySlot{slot}
}

func (dependencySlot) Key() daemon.DependencyKey {
	return dependencyKey{}
}

func (s dependencySlot) Assign(value interface{}) {
	*s.slot = value.(store.Store)
}

type dependencyConfig struct {
	client etcdutil.Client
}

func (k dependencyKey) MakeConfig() daemon.DependencyConfig {
	return &dependencyConfig{}
}

func (cf *dependencyConfig) Populate(deps *daemon.Dependencies) {
	deps.Dependency(etcdutil.ClientDependency(&cf.client))
}

func (cf *dependencyConfig) MakeValue() (interface{}, error) {
	return New(cf.client), nil
}
