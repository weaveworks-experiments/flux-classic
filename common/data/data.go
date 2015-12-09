package data

type AddressSpec struct {
	Type string
	Port int
}

type Selector map[string]string

func (sel Selector) Empty() bool {
	return len(sel) == 0
}

type InstanceSpec struct {
	AddressSpec AddressSpec `json:"addressSpec,omitempty"`
	Selector    Selector    `json:"selector,omitempty"`
}

type InstanceGroup string

type Service struct {
	Address       string                         `json:"address,omitempty"`
	Port          int                            `json:"port,omitempty"`
	Protocol      string                         `json:"protocol,omitempty"`
	InstanceSpecs map[InstanceGroup]InstanceSpec `json:"instanceSpecs,omitempty"`
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

func (spec *InstanceSpec) Includes(s Labeled) bool {
	return spec.Selector.Includes(s)
}

type ServiceChange struct {
	Name    string
	Deleted bool
}
