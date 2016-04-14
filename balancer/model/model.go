package model

import (
	"bytes"
	"fmt"

	"github.com/weaveworks/flux/common/netutil"
)

type Instance struct {
	Name    string
	Address netutil.IPPort
}

type Service struct {
	Name string
	// Protocol, e.g. "http".  "" for simple tcp forwarding.
	Protocol  string
	Address   *netutil.IPPort
	Instances []Instance
}

func (svc *Service) Summary() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s %s/%s {", svc.Name, svc.Address, svc.Protocol)

	comma := ""
	for _, inst := range svc.Instances {
		fmt.Fprintf(&buf, "%s%s %s", comma, inst.Name, inst.Address)
		comma = ", "
	}

	buf.WriteString("}")
	return buf.String()
}

func (a *Service) Equal(b *Service) bool {
	if a.Name != b.Name || a.Protocol != b.Protocol ||
		(a.Address == nil) != (b.Address == nil) ||
		!a.Address.Equal(*b.Address) {
		return false
	}

	type instKey struct {
		name string
		ip   string
		port int
	}

	key := func(i *Instance) instKey {
		return instKey{i.Name, string(i.Address.IP), i.Address.Port}
	}

	m := make(map[instKey]struct{})

	for i := range a.Instances {
		m[key(&a.Instances[i])] = struct{}{}
	}

	for i := range b.Instances {
		k := key(&b.Instances[i])
		if _, found := m[k]; !found {
			return false
		}

		delete(m, k)
	}

	return len(m) == 0
}

type ServiceUpdate struct {
	Updates map[string]*Service
	Reset   bool
}
