package etcdstore

import (
	"time"

	"github.com/weaveworks/flux/common/daemon"
	etcd "github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/heartbeat"
)

type EtcdStore struct {
	// configuration
	client etcd.Client
	ttl    time.Duration
	*etcdStore
}

func (store *EtcdStore) StartFunc() daemon.StartFunc {
	// the restart interval is set so that it will try at least once
	// before records expire.
	hb := &heartbeat.HeartbeatConfig{
		Cluster: store,
		TTL:     store.ttl,
	}

	return daemon.Aggregate(
		daemon.Restart(store.ttl/2, hb.StartFunc()),
		// the interval for the collection is somewhat arbitrary
		daemon.Restart(store.ttl*2, daemon.Ticker(store.ttl*2, func(errs daemon.ErrorSink) {
			errs.Post(store.doCollection())
		})))
}
