package store

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
)

type QueryServiceOptions struct {
	WithInstances      bool
	WithContainerRules bool
}

type ServiceInfo struct {
	Service
	Instances      map[string]Instance
	ContainerRules map[string]ContainerRule
}

type Store interface {
	Cluster

	Ping() error

	RegisterHost(identity string, details *Host) error
	DeregisterHost(identity string) error
	GetHosts() ([]*Host, error)
	WatchHosts(ctx context.Context, resCh chan<- HostChange, errorSink daemon.ErrorSink)

	CheckRegisteredService(serviceName string) error
	AddService(name string, service Service) error
	RemoveService(serviceName string) error
	RemoveAllServices() error

	GetService(serviceName string, opts QueryServiceOptions) (*ServiceInfo, error)
	GetAllServices(opts QueryServiceOptions) (map[string]*ServiceInfo, error)

	SetContainerRule(serviceName string, ruleName string, spec ContainerRule) error
	RemoveContainerRule(serviceName string, ruleName string) error

	AddInstance(serviceName, instanceName string, details Instance) error
	RemoveInstance(serviceName, instanceName string) error

	WatchServices(ctx context.Context, resCh chan<- ServiceChange, errorSink daemon.ErrorSink, opts QueryServiceOptions)
}

// A store that is used as a component
type StoreComponent interface {
	Store
	// block the current goroutine until the store is started
	WaitUntilStarted()
}

// Inline this to get trvial implementations of the StoreComponent methods
type StoreComponentDefault struct {
}

func (_ StoreComponentDefault) WaitUntilStarted() {
	return
}
