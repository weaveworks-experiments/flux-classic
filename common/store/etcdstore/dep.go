package etcdstore

import (
	"time"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/store"
)

type dependencySlot struct {
	slot *store.RuntimeStore
}

type dependencyKey struct{}

func StoreDependency(slot *store.RuntimeStore) daemon.DependencySlot {
	return dependencySlot{slot}
}

func (dependencySlot) Key() daemon.DependencyKey {
	return dependencyKey{}
}

func (s dependencySlot) Assign(value interface{}) {
	*s.slot = value.(store.RuntimeStore)
}

type dependencyConfig struct {
	ttl    int
	client etcdutil.Client
}

func (k dependencyKey) MakeConfig() daemon.DependencyConfig {
	return &dependencyConfig{}
}

func (cf *dependencyConfig) Populate(deps *daemon.Dependencies) {
	deps.IntVar(&cf.ttl, "host-ttl", 30, "The daemon will give its records this time-to-live in seconds, and refresh them while it is running")
	deps.Dependency(etcdutil.ClientDependency(&cf.client))
}

func (cf *dependencyConfig) MakeValue() (interface{}, error) {

	store := &EtcdStore{
		ttl:       time.Duration(cf.ttl) * time.Second,
		client:    cf.client,
		etcdStore: newEtcdStore(cf.client),
	}
	return store, nil
}
