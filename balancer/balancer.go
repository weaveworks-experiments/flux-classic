package balancer

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"

	"github.com/squaremo/ambergreen/balancer/etcdcontrol"
	"github.com/squaremo/ambergreen/balancer/eventlogger"
	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/balancer/prometheus"
	"github.com/squaremo/ambergreen/balancer/simplecontrol"
	"github.com/squaremo/ambergreen/common/daemon"
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

type Controller interface {
	Updates() <-chan model.ServiceUpdate
	Close()
}

type BalancerDaemon struct {
	errorSink    daemon.ErrorSink
	ipTables     *ipTables
	netConfig    netConfig
	controller   Controller
	eventHandler events.Handler
	services     *services
}

func Start(args []string, errorSink daemon.ErrorSink, ipTablesCmd IPTablesCmd) *BalancerDaemon {
	d := &BalancerDaemon{errorSink: errorSink}
	err := d.start(args, ipTablesCmd)
	if err != nil {
		errorSink.Post(err)
	}

	return d
}

func (d *BalancerDaemon) start(args []string, ipTablesCmd IPTablesCmd) error {
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)

	var useSimpleControl bool
	var exposePrometheus string

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	fs.StringVar(&d.netConfig.bridge,
		"bridge", "docker0", "bridge device")
	fs.StringVar(&d.netConfig.chain,
		"chain", "AMBERGREEN", "iptables chain name")
	fs.StringVar(&exposePrometheus,
		"expose-prometheus", "",
		"expose stats to Prometheus on this IPaddress and port; e.g., :9000")
	fs.BoolVar(&useSimpleControl,
		"s", false, "use the unix socket controller")
	fs.Parse(args[1:])

	if fs.NArg() > 0 {
		return fmt.Errorf("excess command line arguments")
	}

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

	if useSimpleControl {
		d.controller, err = simplecontrol.NewServer(d.errorSink)
	} else {
		d.controller, err = etcdcontrol.NewListener(d.errorSink)
	}
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
