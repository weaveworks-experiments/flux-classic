package balancer

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"

	"github.com/squaremo/flux/balancer/etcdcontrol"
	"github.com/squaremo/flux/balancer/eventlogger"
	"github.com/squaremo/flux/balancer/events"
	"github.com/squaremo/flux/balancer/model"
	"github.com/squaremo/flux/balancer/prometheus"
	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/store/etcdstore"
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
	errorSink    daemon.ErrorSink
	ipTables     *ipTables
	netConfig    netConfig
	controller   model.Controller
	eventHandler events.Handler
	services     *services
}

func StartBalancer(args []string, errorSink daemon.ErrorSink, ipTablesCmd IPTablesCmd) *BalancerDaemon {
	d := &BalancerDaemon{errorSink: errorSink}
	err := d.start(args, ipTablesCmd)
	if err != nil {
		errorSink.Post(err)
	}

	return d
}

func (d *BalancerDaemon) start(args []string, ipTablesCmd IPTablesCmd) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)

	var exposePrometheus string
	var debug bool

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	fs.StringVar(&d.netConfig.bridge,
		"bridge", "docker0", "bridge device")
	fs.StringVar(&d.netConfig.chain,
		"chain", "FLUX", "iptables chain name")
	fs.StringVar(&exposePrometheus,
		"expose-prometheus", "",
		"expose stats to Prometheus on this IPaddress and port; e.g., :9000")
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

	d.ipTables = newIPTables(d.netConfig, ipTablesCmd)
	err := d.ipTables.start()
	if err != nil {
		return err
	}

	if exposePrometheus == "" {
		d.eventHandler = eventlogger.EventLogger{}
	} else {
		handler, err := prometheus.NewEventHandler(exposePrometheus)
		if err != nil {
			return err
		}
		d.eventHandler = handler
	}

	store, err := etcdstore.NewFromEnv()
	if err != nil {
		return err
	}

	d.controller, err = etcdcontrol.NewListener(store, d.errorSink)
	if err != nil {
		return err
	}

	d.services = servicesConfig{
		netConfig:    d.netConfig,
		updates:      d.controller.Updates(),
		eventHandler: d.eventHandler,
		ipTables:     d.ipTables,
		errorSink:    d.errorSink,
	}.new()
	return nil
}

func (d *BalancerDaemon) Stop() {
	if d.controller != nil {
		d.controller.Close()
	}

	if d.services != nil {
		d.services.close()
	}

	if d.ipTables != nil {
		d.ipTables.close()
	}
}
