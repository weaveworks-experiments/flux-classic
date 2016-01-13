package store

import (
	"github.com/squaremo/flux/common/data"
)

type InstanceFunc func(serviceName, instanceName string, inst data.Instance) error
type ServiceFunc func(serviceName string, svc data.Service) error

func ForeachServiceInstance(store Store, fs ServiceFunc, fi InstanceFunc) error {
	opts := QueryServiceOptions{WithInstances: fi != nil}
	svcs, err := store.GetAllServices(opts)
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
				if err := fi(s.Name, inst.Name, inst); err != nil {
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
		if err := fi(svc.Name, inst.Name, inst); err != nil {
			return err
		}
	}
	return nil
}

func SelectInstances(store Store, sel data.Selector, fun InstanceFunc) error {
	return ForeachServiceInstance(store, nil, func(sn, in string, d data.Instance) error {
		if sel.Includes(d) {
			if err := fun(sn, in, d); err != nil {
				return err
			}
		}
		return nil
	})
}

func SelectServiceInstances(store Store, service string, s data.Selector, fi InstanceFunc) error {
	return ForeachInstance(store, service, func(sn, in string, d data.Instance) error {
		if s.Includes(d) {
			if err := fi(sn, in, d); err != nil {
				return err
			}
		}
		return nil
	})
}
