package balancer

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"net"
	"sync"

	"github.com/squaremo/ambergreen/balancer/etcdcontrol"
	"github.com/squaremo/ambergreen/balancer/eventlogger"
	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/fatal"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/balancer/prometheus"
	"github.com/squaremo/ambergreen/balancer/simplecontrol"
)

type IPTablesFunc func([]string) ([]byte, error)

type config struct {
	chain        string
	bridge       string
	eventHandler events.Handler
	iptables     IPTablesFunc
}

type Controller interface {
	Updates() <-chan model.ServiceUpdate
	Close()
}

type Interceptor struct {
	fatalSink fatal.Sink

	config           config
	natChainSetup    bool
	filterChainSetup bool
	controller       Controller
	updater          *updater
}

func Start(args []string, fatalSink fatal.Sink, iptables IPTablesFunc) *Interceptor {
	i := &Interceptor{
		fatalSink: fatalSink,
		config:    config{iptables: iptables},
	}
	err := i.start(args)
	if err != nil {
		fatalSink.Post(err)
	}

	return i
}

func (i *Interceptor) start(args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ExitOnError)

	var useSimpleControl bool
	var exposePrometheus string

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	fs.StringVar(&i.config.bridge,
		"bridge", "docker0", "bridge device")
	fs.StringVar(&i.config.chain,
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

	if exposePrometheus == "" {
		i.config.eventHandler = eventlogger.EventLogger{}
	} else {
		handler, err := prometheus.NewEventHandler(exposePrometheus)
		if err != nil {
			return err
		}
		i.config.eventHandler = handler
	}

	err := i.config.setupChain("nat", "PREROUTING")
	if err != nil {
		return err
	}
	i.natChainSetup = true

	err = i.config.setupChain("filter", "FORWARD", "INPUT")
	if err != nil {
		return err
	}
	i.filterChainSetup = true

	if useSimpleControl {
		i.controller, err = simplecontrol.NewServer(i.fatalSink)
	} else {
		i.controller, err = etcdcontrol.NewListener()
	}
	if err != nil {
		return err
	}

	i.updater = i.config.newUpdater(i.controller.Updates(), i.fatalSink)
	return nil
}

func (i *Interceptor) Stop() {
	if i.natChainSetup {
		i.config.deleteChain("nat", "PREROUTING")
	}

	if i.filterChainSetup {
		i.config.deleteChain("filter", "FORWARD", "INPUT")
	}

	if i.controller != nil {
		i.controller.Close()
	}

	if i.updater != nil {
		i.updater.close()
	}
}

type updater struct {
	config    *config
	updates   <-chan model.ServiceUpdate
	fatalSink fatal.Sink

	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
	services map[model.ServiceKey]*service
}

func (config *config) newUpdater(updates <-chan model.ServiceUpdate, fatalSink fatal.Sink) *updater {
	upd := &updater{
		config:    config,
		updates:   updates,
		fatalSink: fatalSink,
		closed:    make(chan struct{}),
		finished:  make(chan struct{}),
		services:  make(map[model.ServiceKey]*service),
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

		svc, err := upd.config.newService(update, upd.fatalSink)
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
	config    *config
	key       model.ServiceKey
	fatalSink fatal.Sink
	state     serviceState

	// No locking, because all operations are called only from the
	// updater goroutine.
}

type serviceState interface {
	stop()
	update(model.ServiceUpdate) (bool, error)
}

func (config *config) newService(upd model.ServiceUpdate, fatalSink fatal.Sink) (*service, error) {
	svc := &service{
		config:    config,
		key:       upd.ServiceKey,
		fatalSink: fatalSink,
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

	err := svc.config.addRule("filter", rule)
	if err != nil {
		return nil, err
	}

	return rejecting(func() {
		svc.config.deleteRule("filter", rule)
	}), nil
}

func (rej rejecting) stop() {
	rej()
}

func (rej rejecting) update(upd model.ServiceUpdate) (bool, error) {
	return len(upd.Instances) == 0, nil
}

func (cf *config) bridgeIP() (net.IP, error) {
	iface, err := net.InterfaceByName(cf.bridge)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if cidr, ok := addr.(*net.IPNet); ok {
			if ip := cidr.IP.To4(); ip != nil {
				return ip, nil
			}
		}
	}

	return nil, fmt.Errorf("no IPv4 address found on netdev %s", cf.bridge)
}