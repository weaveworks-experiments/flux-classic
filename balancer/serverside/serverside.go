package serverside

import (
	"net"
	"time"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/forwarder"
	"github.com/weaveworks/flux/balancer/prometheus"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
)

type Config struct {
	// Should be pre-set
	InstanceUpdates      <-chan agent.InstanceUpdate
	InstanceUpdatesReset chan<- struct{}

	// Just for testing
	serviceUpdates      <-chan store.ServiceUpdate
	serviceUpdatesReset chan<- struct{}

	// From flags/dependencies
	store        store.Store
	eventHandler events.Handler
	hostIP       net.IP
}

func (cf *Config) Populate(deps *daemon.Dependencies) {
	deps.Dependency(etcdstore.StoreDependency(&cf.store))
	deps.Dependency(prometheus.EventHandlerDependency(&cf.eventHandler))
	deps.Dependency(netutil.HostIPDependency(&cf.hostIP))
}

type serverSide struct {
	*Config
	errs daemon.ErrorSink

	services map[string]*service
}

type service struct {
	service   *store.Service
	forwarder *forwarder.Forwarder
	addr      netutil.IPPort
	instances map[string]netutil.IPPort
}

func (cf *Config) Prepare() (daemon.StartFunc, error) {
	startFuncs := []daemon.StartFunc{daemon.SimpleComponent(cf.run)}

	if cf.serviceUpdates == nil {
		serviceUpdates := make(chan store.ServiceUpdate)
		serviceUpdatesReset := make(chan struct{}, 1)
		cf.serviceUpdates = serviceUpdates
		cf.serviceUpdatesReset = serviceUpdatesReset

		startFuncs = append(startFuncs,
			daemon.Reset(serviceUpdatesReset,
				daemon.Restart(10*time.Second,
					store.WatchServicesStartFunc(cf.store,
						store.QueryServiceOptions{},
						serviceUpdates))))
	}

	return daemon.Aggregate(startFuncs...), nil
}

func (cf *Config) run(stop <-chan struct{}, errs daemon.ErrorSink) {
	ss := serverSide{
		Config:   cf,
		errs:     errs,
		services: make(map[string]*service),
	}

	select {
	case ss.InstanceUpdatesReset <- struct{}{}:
	default:
	}

	select {
	case ss.serviceUpdatesReset <- struct{}{}:
	default:
	}

	for {
		select {
		case update := <-ss.serviceUpdates:
			ss.processServiceUpdate(update)

		case update := <-ss.InstanceUpdates:
			if update.Reset {
				ss.processInstanceReset(update)
			} else {
				ss.processInstanceUpdate(update)
			}

		case <-stop:
			return
		}
	}
}

func (ss *serverSide) processServiceUpdate(update store.ServiceUpdate) {
	for svcName, usvc := range update.Services {
		svc := ss.getService(svcName)
		if usvc != nil {
			svc.service = &usvc.Service
		} else {
			svc.service = nil
		}

		ss.updateService(svcName, svc)
	}

	if update.Reset {
		for svcName, svc := range ss.services {
			if update.Services[svcName] == nil {
				svc.service = nil
				ss.updateService(svcName, svc)
			}
		}
	}
}

func (ss *serverSide) processInstanceReset(update agent.InstanceUpdate) {
	svcs := make(map[string]map[string]netutil.IPPort)
	for key, inst := range update.Instances {
		if inst.Address == nil {
			continue
		}

		insts := svcs[key.Service]
		if insts == nil {
			insts = make(map[string]netutil.IPPort)
			svcs[key.Service] = insts
		}

		insts[key.Instance] = *inst.Address
	}

	for svcName, insts := range svcs {
		svc := ss.getService(svcName)
		svc.instances = insts
		ss.updateService(svcName, svc)
	}

	for svcName, svc := range ss.services {
		if svcs[svcName] == nil {
			svc.instances = make(map[string]netutil.IPPort)
			ss.updateService(svcName, svc)
		}
	}
}

func (ss *serverSide) processInstanceUpdate(update agent.InstanceUpdate) {
	for key, inst := range update.Instances {
		svc := ss.getService(key.Service)

		if inst == nil {
			delete(svc.instances, key.Instance)
			ss.updateService(key.Service, svc)
		} else if inst.Address != nil {
			svc.instances[key.Instance] = *inst.Address
			ss.updateService(key.Service, svc)
		}
	}
}

func (ss *serverSide) getService(svcName string) *service {
	svc := ss.services[svcName]
	if svc == nil {
		svc = &service{instances: make(map[string]netutil.IPPort)}
		ss.services[svcName] = svc
	}

	return svc
}

func (ss *serverSide) updateService(svcName string, svc *service) {
	if svc.service != nil && len(svc.instances) != 0 {
		if svc.forwarder == nil {
			fwd, err := forwarder.Config{
				ServiceName:  svcName,
				Description:  svcName,
				EventHandler: ss.eventHandler,
				ErrorSink:    ss.errs,
			}.New()
			if err != nil {
				ss.errs.Post(err)
				return
			}

			fwd.SetProtocol(svc.service.Protocol)
			svc.forwarder = fwd
			svc.addr = netutil.NewIPPort(ss.hostIP, fwd.Addr().Port)
		}

		svc.forwarder.SetInstances(svc.instances)
		ss.errs.Post(ss.store.AddIngressInstance(svcName, svc.addr,
			store.IngressInstance{Weight: len(svc.instances)}))
	} else {
		if svc.forwarder != nil {
			svc.forwarder.Stop()
			svc.forwarder = nil
			ss.errs.Post(ss.store.RemoveIngressInstance(svcName,
				svc.addr))
		}

		if svc.service == nil && len(svc.instances) == 0 {
			delete(ss.services, svcName)
		}
	}
}
