package agent

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store"
)

type HeartbeatConfig struct {
	Cluster      store.Cluster
	TTL          time.Duration
	HostIdentity string
	HostState    *data.Host
}

type Heart struct {
	config HeartbeatConfig
	ticker *time.Ticker
	cancel chan struct{}
}

func NewHeart(config HeartbeatConfig) *Heart {
	h := Heart{config: config}
	return &h
}

func (heart *Heart) Run(errorSink daemon.ErrorSink) {
	heart.cancel = make(chan struct{})
	heart.ticker = time.NewTicker(heart.config.TTL / 2)
	for {
		select {
		case t := <-heart.ticker.C:
			if err := heart.config.Cluster.Heartbeat(heart.config.HostIdentity, heart.config.TTL, heart.config.HostState); err != nil {
				errorSink.Post(err)
				return
			}
			log.Infof("Heartbeat sent at %s", t.String())
		case <-heart.cancel:
			return
		}
	}
}

func (heart *Heart) Start(errorSink daemon.ErrorSink) daemon.Component {
	go heart.Run(errorSink)
	return heart
}

func (heart *Heart) Stop() {
	if heart.ticker != nil {
		heart.ticker.Stop()
	}
	if heart.cancel != nil {
		close(heart.cancel)
	}
}
