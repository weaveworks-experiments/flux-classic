package store

import (
	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/data"
)

type WatchServicesOptions struct {
	WithInstanceChanges  bool
	WithGroupSpecChanges bool
}

type QueryServiceOptions struct {
	WithInstances  bool
	WithGroupSpecs bool
}

type Store interface {
	Ping() error

	CheckRegisteredService(serviceName string) error
	AddService(name string, service data.Service) error
	RemoveService(serviceName string) error
	RemoveAllServices() error

	GetService(serviceName string, opts QueryServiceOptions) (data.Service, error)
	GetAllServices(opts QueryServiceOptions) ([]data.Service, error)

	SetContainerGroupSpec(serviceName string, groupName string, spec data.ContainerGroupSpec) error
	RemoveContainerGroupSpec(serviceName string, groupName string) error

	AddInstance(serviceName, instanceName string, details data.Instance) error
	RemoveInstance(serviceName, instanceName string) error

	WatchServices(resCh chan<- data.ServiceChange, stopCh <-chan struct{}, errorSink daemon.ErrorSink, opts WatchServicesOptions)
}
