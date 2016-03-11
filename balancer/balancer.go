package balancer

import (
	"flag"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer/eventlogger"
	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/balancer/prometheus"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
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

type BalancerDaemon struct {
	errorSink   daemon.ErrorSink
	ipTablesCmd IPTablesCmd

	// From flags
	updates      <-chan model.ServiceUpdate
	controller   daemon.Component
	eventHandler events.Handler
	netConfig    netConfig
	done         chan<- model.ServiceUpdate

	ipTables *ipTables
	services *services
}

func (d *BalancerDaemon) parseArgs(args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)

	var promCf prometheus.Config
	var debug bool

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	fs.StringVar(&d.netConfig.bridge,
		"bridge", "docker0", "bridge device")
	fs.StringVar(&d.netConfig.chain,
		"chain", "FLUX", "iptables chain name")
	fs.StringVar(&promCf.ListenAddr,
		"listen-prometheus", "",
		"listen for connections from Prometheus on this IP address and port; e.g., :9000")
	fs.StringVar(&promCf.AdvertiseAddr,
		"advertise-prometheus", "",
		"IP address and port to advertise to Prometheus; e.g. 192.168.42.221:9000")

	fs.BoolVar(&debug, "debug", false, "output debugging logs")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("excess command line arguments")
	}

	if debug {
		log.SetLevel(log.DebugLevel)
	}
	log.Debug("Debug logging on")

	etcdclient, err := etcdutil.NewClientFromEnv()
	if err != nil {
		return err
	}

	d.setStore(etcdstore.New(etcdclient), 10*time.Second)

	if promCf.ListenAddr == "" {
		if promCf.AdvertiseAddr != "" {
			return fmt.Errorf("-advertise-prometheus option must be accompanied by -listen-prometheus")
		}

		d.eventHandler = eventlogger.EventLogger{}
	} else {
		if promCf.AdvertiseAddr == "" {
			promCf.AdvertiseAddr = promCf.ListenAddr
		}

		promCf.ErrorSink = d.errorSink
		promCf.EtcdClient = etcdclient
		handler, err := prometheus.NewEventHandler(promCf)
		if err != nil {
			return err
		}
		d.eventHandler = handler
	}

	return nil
}

func (d *BalancerDaemon) setStore(st store.Store, reconnectInterval time.Duration) {
	updates := make(chan model.ServiceUpdate)
	d.updates = updates
	d.controller = daemon.Restart(reconnectInterval, model.WatchServicesStartFunc(st, updates))(d.errorSink)
}

func NewBalancer(args []string, errorSink daemon.ErrorSink, ipTablesCmd IPTablesCmd) (*BalancerDaemon, error) {
	d := &BalancerDaemon{
		errorSink:   errorSink,
		ipTablesCmd: ipTablesCmd,
	}

	if err := d.parseArgs(args); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *BalancerDaemon) Start() {
	d.ipTables = newIPTables(d.netConfig, d.ipTablesCmd)
	if err := d.ipTables.start(); err != nil {
		d.errorSink.Post(err)
		return
	}

	d.services = servicesConfig{
		netConfig:    d.netConfig,
		updates:      d.updates,
		eventHandler: d.eventHandler,
		ipTables:     d.ipTables,
		errorSink:    d.errorSink,
		done:         d.done,
	}.start()

	d.eventHandler.Start()
}

func (d *BalancerDaemon) Stop() {
	d.eventHandler.Stop()

	if controller := d.controller; controller != nil {
		d.controller = nil
		controller.Stop()
	}

	if services := d.services; services != nil {
		d.services = nil
		services.close()
	}

	if ipTables := d.ipTables; ipTables != nil {
		d.ipTables = nil
		ipTables.close()
	}
}
