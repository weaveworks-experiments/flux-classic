package heartbeat

import (
	"time"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type HeartbeatConfig struct {
	Cluster store.Cluster
	TTL     time.Duration
}

func (config HeartbeatConfig) StartFunc() daemon.StartFunc {
	return daemon.Ticker(config.TTL/2, func(errs daemon.ErrorSink) {
		errs.Post(config.Cluster.Heartbeat(config.TTL))
	})
}
