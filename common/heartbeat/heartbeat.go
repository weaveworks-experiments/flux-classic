package heartbeat

import (
	"sync"
	"time"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type HeartbeatConfig struct {
	Cluster store.Cluster
	TTL     time.Duration
	Started chan<- struct{}
}

func (config HeartbeatConfig) StartFunc() daemon.StartFunc {
	first := sync.Once{}
	return daemon.Ticker(config.TTL/2, func(errs daemon.ErrorSink) {
		first.Do(func() {
			if config.Started != nil {
				close(config.Started)
			}
		})
		errs.Post(config.Cluster.Heartbeat(config.TTL))
	})
}
