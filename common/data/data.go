package data

type Selector map[string]string

func (sel Selector) Empty() bool {
	return len(sel) == 0
}

// Specifies which containers form an instance group, and the
// attributes of the resulting instances.
type InstanceGroupSpec struct {
	AddressSpec AddressSpec `json:"addressSpec,omitempty"`
	Selector    Selector    `json:"selector,omitempty"`
}

const MAPPED = "mapped"
const FIXED = "fixed"

type AddressSpec struct {
	// Type is "mapped" or "fixed".
	//
	// "mapped" means that the instance address is formed from the
	// host IP (as passed to the agent), and the host port number
	// associated by docker's port mapping with the container port
	// given by Port.  As a consequence, it allows cross-host
	// operation without a multi-host container network: On a
	// client host, a connection to the service address is
	// directed to an instance host, and crosses the network; on
	// the instance host, docker directs the connection to the
	// instance container.
	//
	// "fixed" means that the instance address is formed from the
	// given Port and the container IP address as reported by
	// docker.  So this mode only allows single host operation,
	// unless a multi-host container network is in use.
	Type string

	// The port number of the instance within the target
	// container.
	Port int
}

type InstanceGroup string

type Service struct {
	Address            string                              `json:"address,omitempty"`
	Port               int                                 `json:"port,omitempty"`
	Protocol           string                              `json:"protocol,omitempty"`
	InstanceGroupSpecs map[InstanceGroup]InstanceGroupSpec `json:"instanceSpecs,omitempty"`
}

type Instance struct {
	OwnerID       string            `json:"ownerID"`
	InstanceGroup InstanceGroup     `json:"instanceGroup"`
	Address       string            `json:"address,omitempty"`
	Port          int               `json:"port,omitempty"`
	Labels        map[string]string `json:"labels"`
}

type Labeled interface {
	Label(string) string
}

func (inst Instance) Label(k string) string {
	return inst.Labels[k]
}

func (sel Selector) Includes(s Labeled) bool {
	for label, value := range sel {
		if s.Label(label) != value {
			return false
		}
	}
	return true
}

func (spec *InstanceGroupSpec) Includes(s Labeled) bool {
	return spec.Selector.Includes(s)
}

type ServiceChange struct {
	Name           string
	ServiceDeleted bool
}
