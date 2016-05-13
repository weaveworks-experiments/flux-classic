package store

import (
	"net"

	"github.com/weaveworks/flux/common/netutil"
)

type Selector map[string]string

func (sel Selector) Empty() bool {
	return len(sel) == 0
}

type Host struct {
	IP net.IP `json:"address"`
}

type HostChange struct {
	Name         string
	HostDeparted bool
}

// Specifies how containers should be selected as instances, and the
// attributes of the resulting instances.
type ContainerRule struct {
	Selector     Selector `json:"selector,omitempty"`
	InstancePort int      `json:"instancePort,omitempty"`
}

type Service struct {
	Address      *netutil.IPPort `json:"address,omitempty"`
	InstancePort int             `json:"instancePort,omitempty"`
	Protocol     string          `json:"protocol,omitempty"`
}

type ServiceInfo struct {
	Service
	Instances        map[string]Instance
	ContainerRules   map[string]ContainerRule
	IngressInstances map[netutil.IPPort]IngressInstance
}

const (
	HostLabel  = "host"
	StateLabel = "state"
	RuleLabel  = "rule"
)

type Instance struct {
	Host          Host              `json:"host"`
	ContainerRule string            `json:"containerRule"`
	Address       *netutil.IPPort   `json:"address,omitempty"`
	Labels        map[string]string `json:"labels"`
}

type IngressInstance struct {
	Weight int `json:"weight"`
}

type Labeled interface {
	Label(string) string
}

func (inst Instance) Label(k string) string {
	switch k {
	case HostLabel:
		return inst.Host.IP.String()
	case StateLabel:
		if inst.Address == nil {
			return "no address"
		} else {
			return "live"
		}
	case RuleLabel:
		return inst.ContainerRule
	default:
		return inst.Labels[k]
	}
}

func (sel Selector) Includes(s Labeled) bool {
	for label, value := range sel {
		if s.Label(label) != value {
			return false
		}
	}
	return true
}

func (spec *ContainerRule) Includes(s Labeled) bool {
	return spec.Selector.Includes(s)
}

type ServiceChange struct {
	Name           string
	ServiceDeleted bool
}
