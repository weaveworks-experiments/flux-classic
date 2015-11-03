package interceptor

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/squaremo/ambergreen/balancer/coatl"
	"github.com/squaremo/ambergreen/balancer/interceptor/eventlogger"
	"github.com/squaremo/ambergreen/balancer/interceptor/events"
	"github.com/squaremo/ambergreen/balancer/interceptor/model"
	"github.com/squaremo/ambergreen/balancer/interceptor/prometheus"
	"github.com/squaremo/ambergreen/balancer/interceptor/simplecontrol"
)

type config struct {
	chain        string
	bridge       string
	eventHandler events.Handler
}

type Controller interface {
	Updates() <-chan model.ServiceUpdate
	Close()
}

func Main() error {
	var cf config
	var useSimpleControl bool
	var exposePrometheus string

	// The bridge specified should be the one where packets sent
	// to service IP addresses go.  So even with weave, that's
	// typically 'docker0'.
	flag.StringVar(&cf.bridge, "bridge", "docker0", "bridge device")
	flag.StringVar(&cf.chain, "chain", "AMBERGRIS", "iptables chain name")
	flag.StringVar(&exposePrometheus, "expose-prometheus", "", "expose stats to Prometheus on this IPaddress and port; e.g., :9000")
	flag.BoolVar(&useSimpleControl, "s", false, "use the unix socket controller")
	flag.Parse()

	if flag.NArg() > 0 {
		return fmt.Errorf("excess command line arguments")
	}

	if exposePrometheus == "" {
		cf.eventHandler = eventlogger.EventLogger{}
	} else {
		handler, err := prometheus.NewEventHandler(exposePrometheus)
		if err != nil {
			return err
		}
		cf.eventHandler = handler
	}

	err := cf.setupChain("nat", "PREROUTING")
	if err != nil {
		return err
	}
	defer cf.deleteChain("nat", "PREROUTING")

	err = cf.setupChain("filter", "FORWARD", "INPUT")
	if err != nil {
		return err
	}
	defer cf.deleteChain("filter", "FORWARD", "INPUT")

	errors := make(chan error, 1)

	var controlServer Controller
	if useSimpleControl {
		controlServer, err = simplecontrol.NewServer(errors)
	} else {
		controlServer, err = coatl.NewListener(errors)
	}
	if err != nil {
		return err
	}
	defer controlServer.Close()

	updater := cf.newUpdater(controlServer.Updates(), errors)
	defer updater.close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigs:
	case err := <-errors:
		return err
	}

	return nil
}

type updater struct {
	config  *config
	updates <-chan model.ServiceUpdate
	errors  chan<- error

	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
	services map[model.ServiceKey]*service
}

func (config *config) newUpdater(updates <-chan model.ServiceUpdate, errors chan<- error) *updater {
	upd := &updater{
		config:   config,
		updates:  updates,
		errors:   errors,
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

		svc, err := upd.config.newService(update, upd.errors)
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
	config *config
	key    model.ServiceKey
	errors chan<- error
	state  serviceState

	// No locking, because all operations are called only from the
	// updater goroutine.
}

type serviceState interface {
	stop()
	update(model.ServiceUpdate) (bool, error)
}

func (config *config) newService(upd model.ServiceUpdate, errors chan<- error) (*service, error) {
	svc := &service{
		config: config,
		key:    upd.ServiceKey,
		errors: errors,
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
