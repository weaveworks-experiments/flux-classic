package balancer

import (
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/balancer/prometheus"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
)

func logError(err error, args ...interface{}) {
	if err != nil {
		log.WithError(err).Error(args...)
	}
}

type netConfig struct {
	chain  string
	bridge string
}

type BalancerConfig struct {
	// Should be pre-set
	IPTablesCmd       IPTablesCmd
	done              chan<- model.ServiceUpdate
	reconnectInterval time.Duration

	// From flags/dependencies
	netConfig    netConfig
	debug        bool
	store        store.StoreComponent
	eventHandler events.Handler

	// Filled by Prepare
	updates <-chan model.ServiceUpdate
}

func (cf *BalancerConfig) Populate(deps *daemon.Dependencies) {
	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	deps.StringVar(&cf.netConfig.bridge,
		"bridge", "docker0", "bridge device")
	deps.StringVar(&cf.netConfig.chain,
		"chain", "FLUX", "iptables chain name")
	deps.BoolVar(&cf.debug, "debug", false, "output debugging logs")

	deps.Dependency(etcdstore.StoreDependency(&cf.store))
	deps.Dependency(prometheus.EventHandlerDependency(&cf.eventHandler))
}

func (cf *BalancerConfig) Prepare() (daemon.StartFunc, error) {
	if cf.debug {
		log.SetLevel(log.DebugLevel)
	}
	log.Debug("Debug logging on")

	if cf.reconnectInterval == 0 {
		cf.reconnectInterval = 10 * time.Second
	}

	updates := make(chan model.ServiceUpdate)
	updatesReset := make(chan struct{}, 1)

	startBalancer := func(errs daemon.ErrorSink) daemon.Component {
		b := &balancer{
			cf:      cf,
			errs:    errs,
			updates: updates,
		}

		select {
		case updatesReset <- struct{}{}:
		default:
		}

		errs.Post(b.start())
		return b
	}

	return daemon.Aggregate(
		daemon.Reset(updatesReset,
			daemon.Restart(cf.reconnectInterval,
				model.WatchServicesStartFunc(cf.store, true,
					updates))),

		daemon.Restart(cf.reconnectInterval, startBalancer)), nil
}

type balancer struct {
	cf       *BalancerConfig
	errs     daemon.ErrorSink
	updates  <-chan model.ServiceUpdate
	ipTables *ipTables
	services *services
}

func (b *balancer) start() error {
	b.ipTables = newIPTables(b.cf.netConfig, b.cf.IPTablesCmd)
	if err := b.ipTables.start(); err != nil {
		return err
	}

	b.services = servicesConfig{
		netConfig:    b.cf.netConfig,
		updates:      b.updates,
		eventHandler: b.cf.eventHandler,
		ipTables:     b.ipTables,
		errorSink:    b.errs,
		done:         b.cf.done,
	}.start()

	return nil
}

func (b *balancer) Stop() {
	if b.services != nil {
		b.services.stop()
	}

	b.ipTables.stop()
}
