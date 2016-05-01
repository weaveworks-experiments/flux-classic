package model

import (
	"bytes"
	"fmt"

	"github.com/weaveworks/flux/common/netutil"
)

type Service struct {
	Name string
	// Protocol, e.g. "http".  "" for simple tcp forwarding.
	Protocol  string
	Address   *netutil.IPPort
	Instances map[string]netutil.IPPort // map from name to address
}

func (svc *Service) Summary() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s %s/%s {", svc.Name, svc.Address, svc.Protocol)

	comma := ""
	for name, addr := range svc.Instances {
		fmt.Fprintf(&buf, "%s%s %s", comma, name, addr)
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

	for name, aAddr := range a.Instances {
		bAddr, found := b.Instances[name]
		if !found || !aAddr.Equal(bAddr) {
			return false
		}
	}

	for name := range b.Instances {
		if _, found := a.Instances[name]; !found {
			return false
		}
	}

	return true
}

type ServiceUpdate struct {
	Updates map[string]*Service
	Reset   bool
}
