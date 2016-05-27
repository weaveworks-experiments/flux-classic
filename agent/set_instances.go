package agent

import (
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type setInstancesConfig struct {
	hostIP net.IP
	store  store.Store

	instanceUpdates      <-chan InstanceUpdate
	instanceUpdatesReset chan<- struct{}

	// For testing
	didUpdate chan<- struct{}
}

type setInstances struct {
	setInstancesConfig
	errs daemon.ErrorSink
}

func (conf setInstancesConfig) StartFunc() daemon.StartFunc {
	return daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		// Announce our presence
		conf.store.RegisterHost(conf.hostIP.String(),
			&store.Host{IP: conf.hostIP})

		si := setInstances{
			setInstancesConfig: conf,
			errs:               errs,
		}

		select {
		case si.instanceUpdatesReset <- struct{}{}:
		default:
		}

		for {
			select {
			case update := <-si.instanceUpdates:
				si.processUpdate(update)
				if conf.didUpdate != nil {
					conf.didUpdate <- struct{}{}
				}

			case <-stop:
				return
			}
		}
	})
}

func (si *setInstances) processReset(update InstanceUpdate) {
	// We need to get all services, because we need to prune
	// instances on all services, even ones that we no longer have
	// instances for.
	svcs, err := si.store.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		si.errs.Post(err)
		return
	}

	for svcName, svc := range svcs {
		for instName, inst := range svc.Instances {
			if !si.hostIP.Equal(inst.Host.IP) {
				continue
			}

			key := InstanceKey{
				Service:  svcName,
				Instance: instName,
			}
			if update.Instances[key] == nil {
				si.removeInstance(key)
			}
		}
	}
}

func (si *setInstances) processUpdate(update InstanceUpdate) {
	if update.Reset {
		si.processReset(update)
	}

	for key, inst := range update.Instances {
		if inst == nil {
			si.removeInstance(key)
		} else {
			log.Infof(`Registering service '%s' instance '%.12s' at %s`, key.Service, key.Instance, inst.Address)
			si.errs.Post(si.store.AddInstance(key.Service, key.Instance, *inst))
		}
	}
}

func (si *setInstances) removeInstance(key InstanceKey) {
	log.Infof("Deregistering service '%s' instance '%.12s'", key.Service, key.Instance)
	si.errs.Post(si.store.RemoveInstance(key.Service, key.Instance))
}
