package store

import (
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
)

type QueryServiceOptions struct {
	WithInstances      bool
	WithContainerRules bool
}

type InstanceInfo struct {
	Name string `json:"name"`
	Instance
}

type ContainerRuleInfo struct {
	Name string `json:"name"`
	ContainerRule
}

type ServiceInfo struct {
	Name string `json:"name"`
	Service
	Instances      []InstanceInfo      `json:"instances,omitempty"`
	ContainerRules []ContainerRuleInfo `json:"groups,omitempty"`
}

type Store interface {
	Cluster

	Ping() error

	CheckRegisteredService(serviceName string) error
	AddService(name string, service Service) error
	RemoveService(serviceName string) error
	RemoveAllServices() error

	GetService(serviceName string, opts QueryServiceOptions) (*ServiceInfo, error)
	GetAllServices(opts QueryServiceOptions) ([]*ServiceInfo, error)

	SetContainerRule(serviceName string, ruleName string, spec ContainerRule) error
	RemoveContainerRule(serviceName string, ruleName string) error

	AddInstance(serviceName, instanceName string, details Instance) error
	RemoveInstance(serviceName, instanceName string) error

	WatchServices(ctx context.Context, resCh chan<- ServiceChange, errorSink daemon.ErrorSink, opts QueryServiceOptions)
}
