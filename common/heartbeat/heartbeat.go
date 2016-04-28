package heartbeat

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type HeartbeatConfig struct {
	Cluster store.Cluster
	TTL     time.Duration
}

func (config HeartbeatConfig) StartFunc() daemon.StartFunc {
	return daemon.Ticker(config.TTL/2, config.beat)
}

func (heart *HeartbeatConfig) beat(t time.Time) error {
	log.Debugf("Heartbeat at %s", t)
	return heart.Cluster.Heartbeat(heart.TTL)
}
