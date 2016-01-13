package store

import (
	"github.com/squaremo/flux/common/data"
)

type InstanceFunc func(instanceName string, inst data.Instance) error
type ServiceFunc func(serviceName string, svc data.Service) error

func ForeachServiceInstance(store Store, fs ServiceFunc, fi InstanceFunc) error {
	svcs, err := store.GetAllServices(QueryServiceOptions{WithInstances: true})
	if err != nil {
		return err
	}
	for _, s := range svcs {
		if fs != nil {
			if err := fs(s.Name, s); err != nil {
				return err
			}
		}
		if fi != nil {
			for _, inst := range s.Instances {
				if err := fi(inst.Name, inst); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ForeachInstance(store Store, serviceName string, fi InstanceFunc) error {
	svc, err := store.GetService(serviceName, QueryServiceOptions{WithInstances: true})
	if err != nil {
		return err
	}
	for _, inst := range svc.Instances {
		if err := fi(inst.Name, inst); err != nil {
			return err
		}
	}
	return nil
}

func SelectInstances(store Store, sel data.Selector, fun InstanceFunc) error {
	return ForeachServiceInstance(store, nil, func(i string, d data.Instance) error {
		if sel.Includes(d) {
			if err := fun(i, d); err != nil {
				return err
			}
		}
		return nil
	})
}

func SelectServiceInstances(store Store, service string, s data.Selector, fi InstanceFunc) error {
	return ForeachInstance(store, service, func(i string, d data.Instance) error {
		if s.Includes(d) {
			if err := fi(i, d); err != nil {
				return err
			}
		}
		return nil
	})
}
