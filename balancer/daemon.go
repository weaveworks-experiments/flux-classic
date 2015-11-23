package balancer

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"sync"

	"github.com/squaremo/ambergreen/balancer/etcdcontrol"
	"github.com/squaremo/ambergreen/balancer/eventlogger"
	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/fatal"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/balancer/prometheus"
	"github.com/squaremo/ambergreen/balancer/simplecontrol"
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

type Interceptor struct {
	fatalSink    fatal.Sink
	ipTables     *ipTables
	netConfig    netConfig
	controller   Controller
	eventHandler events.Handler
	updater      *updater
}

func Start(args []string, fatalSink fatal.Sink, ipTablesCmd IPTablesCmd) *Interceptor {
	i := &Interceptor{fatalSink: fatalSink}
	err := i.start(args, ipTablesCmd)
	if err != nil {
		fatalSink.Post(err)
	}

	return i
}

func (i *Interceptor) start(args []string, ipTablesCmd IPTablesCmd) error {
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)

	var useSimpleControl bool
	var exposePrometheus string

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	fs.StringVar(&i.netConfig.bridge,
		"bridge", "docker0", "bridge device")
	fs.StringVar(&i.netConfig.chain,
		"chain", "AMBERGRIS", "iptables chain name")
	fs.StringVar(&exposePrometheus,
		"expose-prometheus", "",
		"expose stats to Prometheus on this IPaddress and port; e.g., :9000")
	fs.BoolVar(&useSimpleControl,
		"s", false, "use the unix socket controller")
	fs.Parse(args[1:])

	if fs.NArg() > 0 {
		return fmt.Errorf("excess command line arguments")
	}

	i.ipTables = newIPTables(i.netConfig, ipTablesCmd)
	err := i.ipTables.start()
	if err != nil {
		return err
	}

	if exposePrometheus == "" {
		i.eventHandler = eventlogger.EventLogger{}
	} else {
		handler, err := prometheus.NewEventHandler(exposePrometheus)
		if err != nil {
			return err
		}
		i.eventHandler = handler
	}

	if useSimpleControl {
		i.controller, err = simplecontrol.NewServer(i.fatalSink)
	} else {
		i.controller, err = etcdcontrol.NewListener()
	}
	if err != nil {
		return err
	}

	i.updater = updaterConfig{
		netConfig:    i.netConfig,
		updates:      i.controller.Updates(),
		eventHandler: i.eventHandler,
		ipTables:     i.ipTables,
		fatalSink:    i.fatalSink,
	}.new()
	return nil
}

func (i *Interceptor) Stop() {
	if i.controller != nil {
		i.controller.Close()
	}

	if i.updater != nil {
		i.updater.close()
	}

	if i.ipTables != nil {
		i.ipTables.close()
	}
}

type updaterConfig struct {
	netConfig netConfig
	updates   <-chan model.ServiceUpdate
	*ipTables
	eventHandler events.Handler
	fatalSink    fatal.Sink
}

type updater struct {
	updaterConfig

	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
	services map[model.ServiceKey]*service
}

func (cf updaterConfig) new() *updater {
	upd := &updater{
		updaterConfig: cf,

		closed:   make(chan struct{}),
		finished: make(chan struct{}),
		services: make(map[model.ServiceKey]*service),
	}
	go upd.run()
	return upd
}

func (upd *updater) close() {
	upd.lock.Lock()
	defer upd.lock.Unlock()

	if upd.services != nil {
		close(upd.closed)
		<-upd.finished

		for _, svc := range upd.services {
			svc.close()
		}

		upd.services = nil
	}
}

func (upd *updater) run() {
	for {
		select {
		case <-upd.closed:
			close(upd.finished)
			return

		case update := <-upd.updates:
			upd.doUpdate(update)
		}
	}
}

func (upd *updater) doUpdate(update model.ServiceUpdate) {
	svc := upd.services[update.ServiceKey]
	if svc == nil {
		if update.ServiceInfo == nil {
			return
		}

		svc, err := upd.newService(update)
		if err != nil {
			log.Error("adding service ", update.ServiceKey, ": ",
				err)
			return
		}

		upd.services[update.ServiceKey] = svc
	} else if update.ServiceInfo != nil {
		err := svc.update(update)
		if err != nil {
			log.Error("updating service ", update.ServiceKey, ": ",
				err)
			return
		}
	} else {
		delete(upd.services, update.ServiceKey)
		svc.close()
	}
}

type service struct {
	*updater
	key   model.ServiceKey
	state serviceState
}

type serviceState interface {
	stop()
	update(model.ServiceUpdate) (bool, error)
}

func (updater *updater) newService(upd model.ServiceUpdate) (*service, error) {
	svc := &service{
		updater: updater,
		key:     upd.ServiceKey,
	}

	err := svc.update(upd)
	if err != nil {
		return nil, err
	}

	return svc, nil
}

func (svc *service) update(upd model.ServiceUpdate) error {
	if svc.state != nil {
		ok, err := svc.state.update(upd)
		if err != nil || ok {
			return err
		}
	}

	// start the new forwarder before stopping the old one, to
	// avoid a window where there is no rule for the service
	start := svc.startForwarding
	if len(upd.Instances) == 0 {
		start = svc.startRejecting
	}

	state, err := start(upd)
	if err != nil {
		return err
	}

	if svc.state != nil {
		svc.state.stop()
	}

	svc.state = state
	return nil
}

func (svc *service) close() {
	svc.state.stop()
	svc.state = nil
}

type rejecting func()

func (svc *service) startRejecting(upd model.ServiceUpdate) (serviceState, error) {
	rule := []interface{}{
		"-p", "tcp",
		"-d", upd.IP(),
		"--dport", upd.Port,
		"-j", "REJECT",
	}

	err := svc.ipTables.addRule("filter", rule)
	if err != nil {
		return nil, err
	}

	return rejecting(func() {
		svc.ipTables.deleteRule("filter", rule)
	}), nil
}

func (rej rejecting) stop() {
	rej()
}

func (rej rejecting) update(upd model.ServiceUpdate) (bool, error) {
	return len(upd.Instances) == 0, nil
}
