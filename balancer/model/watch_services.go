package model

import (
	"net"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

func WatchServicesStartFunc(st store.Store, updates chan<- ServiceUpdate) daemon.StartFunc {
	sendUpdate := func(su store.ServiceUpdate, stop <-chan struct{}) {
		update := make(map[string]*Service)
		for name, svc := range su.Services {
			var ms *Service
			if svc != nil {
				if ms = translateService(svc); ms == nil {
					continue
				}
			}

			update[name] = ms
		}

		select {
		case updates <- ServiceUpdate{
			Updates: update,
			Reset:   su.Reset,
		}:
		case <-stop:
		}
	}
	return store.WatchServicesIndirectStartFunc(st,
		store.QueryServiceOptions{WithInstances: true},
		sendUpdate)
}

func translateService(svc *store.ServiceInfo) *Service {
	var (
		ip   net.IP
		port int
	)

	if svc.Address != nil {
		if svc.Address.IP == nil {
			log.Errorf("Bad address \"%s\" for service %s",
				svc.Address, svc.Name)
			return nil
		}
		ip = svc.Address.IP
		port = svc.Address.Port
	}

	insts := []Instance{}
	for _, instance := range svc.Instances {
		if instance.State != store.LIVE {
			continue // try next instance
		}

		ip := net.ParseIP(instance.Address)
		if ip == nil {
			log.Errorf("Bad address \"%s\" for instance %s/%s",
				instance.Address, svc.Name, instance.Name)
			continue
		}

		insts = append(insts, Instance{
			Name: instance.Name,
			IP:   ip,
			Port: instance.Port,
		})
	}

	return &Service{
		Name:      svc.Name,
		Protocol:  svc.Protocol,
		IP:        ip,
		Port:      port,
		Instances: insts,
	}
}
