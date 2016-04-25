package model

import (
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
	insts := []Instance{}
	for name, instance := range svc.Instances {
		if instance.Address != nil {
			insts = append(insts, Instance{
				Name:    name,
				Address: *instance.Address,
			})
		}
	}

	return &Service{
		Name:      svc.Name,
		Protocol:  svc.Protocol,
		Address:   svc.Address,
		Instances: insts,
	}
}
