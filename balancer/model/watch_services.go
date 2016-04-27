package model

import (
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

func WatchServicesStartFunc(st store.Store, filterAddressless bool, updates chan<- ServiceUpdate) daemon.StartFunc {
	sendUpdate := func(su store.ServiceUpdate, stop <-chan struct{}) {
		update := make(map[string]*Service)
		for name, svc := range su.Services {
			update[name] = translateService(name, svc,
				filterAddressless)
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

func translateService(name string, svc *store.ServiceInfo, filterAddressless bool) *Service {
	if svc == nil || (filterAddressless && svc.Address == nil) {
		return nil
	}

	insts := []Instance{}
	for instName, instance := range svc.Instances {
		if instance.Address != nil {
			insts = append(insts, Instance{
				Name:    instName,
				Address: *instance.Address,
			})
		}
	}

	return &Service{
		Name:      name,
		Protocol:  svc.Protocol,
		Address:   svc.Address,
		Instances: insts,
	}
}
