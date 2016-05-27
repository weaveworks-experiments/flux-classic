package etcdstore

import (
	"time"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/heartbeat"
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

func (cf *dependencyConfig) MakeValue() (interface{}, daemon.StartFunc, error) {
	st := newEtcdStore(cf.client)
	return st, cf.startFunc(st), nil
}

func (cf *dependencyConfig) startFunc(st *etcdStore) daemon.StartFunc {
	// the restart interval is set so that it will try at least once
	// before records expire.
	ttl := time.Duration(cf.ttl) * time.Second
	hb := &heartbeat.HeartbeatConfig{
		Cluster: st,
		TTL:     ttl,
	}

	return daemon.Aggregate(
		daemon.Restart(ttl/2, hb.StartFunc()),
		// the interval for the collection is somewhat arbitrary
		daemon.Restart(ttl*2, daemon.Ticker(ttl*2, func(errs daemon.ErrorSink) {
			errs.Post(st.doCollection())
		})))
}
