package data

type Selector map[string]string

func (sel Selector) Empty() bool {
	return len(sel) == 0
}

// Specifies how containers should be selected as instances, and the
// attributes of the resulting instances.
type ContainerRule struct {
	Selector Selector `json:"selector,omitempty"`
}

type Service struct {
	Address      string `json:"address,omitempty"`
	Port         int    `json:"port,omitempty"`
	InstancePort int    `json:"instancePort,omitempty"`
	Protocol     string `json:"protocol,omitempty"`
}

type InstanceState string

const (
	LIVE   InstanceState = "live"
	NOADDR InstanceState = "no address"
)

type Instance struct {
	State         InstanceState     `json:"state"`
	OwnerID       string            `json:"ownerID"`
	ContainerRule string            `json:"containerRule"`
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

func (spec *ContainerRule) Includes(s Labeled) bool {
	return spec.Selector.Includes(s)
}

type ServiceChange struct {
	Name           string
	ServiceDeleted bool
}
