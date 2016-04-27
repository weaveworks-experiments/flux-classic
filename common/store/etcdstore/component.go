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

	// used when started
	heartbeatConfig heartbeat.HeartbeatConfig
}

func (store *EtcdStore) StartFunc() daemon.StartFunc {
	// the restart interval is set so that it will try at least once
	// before records expire.
	store.heartbeatConfig = heartbeat.HeartbeatConfig{
		Cluster: store,
		TTL:     store.ttl,
	}

	return daemon.Restart(store.ttl/2, store.heartbeatConfig.Start)
}
