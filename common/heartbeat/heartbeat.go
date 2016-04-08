package heartbeat

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type HeartbeatConfig struct {
	Cluster      store.Cluster
	TTL          time.Duration
	HostIdentity string
	HostState    *store.Host
}

func (config HeartbeatConfig) Start(errorSink daemon.ErrorSink) daemon.Component {
	heart := Heart{
		config: config,
	}
	if err := heart.beat(); err != nil {
		errorSink.Post(err)
		return &heart
	}
	go heart.Run(errorSink)
	return &heart
}

type Heart struct {
	config HeartbeatConfig
	ticker *time.Ticker
	cancel chan struct{}
}

func (heart *Heart) beat() error {
	return heart.config.Cluster.Heartbeat(heart.config.HostIdentity,
		heart.config.TTL, heart.config.HostState)
}

func (heart *Heart) Run(errorSink daemon.ErrorSink) {
	heart.cancel = make(chan struct{})
	heart.ticker = time.NewTicker(heart.config.TTL / 2)
	for {
		select {
		case t := <-heart.ticker.C:
			if err := heart.beat(); err != nil {
				errorSink.Post(err)
				return
			}
			log.Infof("Heartbeat sent at %s", t.String())
		case <-heart.cancel:
			return
		}
	}
}

func (heart *Heart) Stop() {
	if heart.ticker != nil {
		heart.ticker.Stop()
	}
	if heart.cancel != nil {
		heart.cancel <- struct{}{}
	}
}
