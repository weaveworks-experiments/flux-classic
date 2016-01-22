package model

import (
	"bytes"
	"fmt"
	"net"
)

type Instance struct {
	Name  string
	Group string
	IP    net.IP
	Port  int
}

func (inst *Instance) TCPAddr() *net.TCPAddr {
	return &net.TCPAddr{IP: inst.IP, Port: inst.Port}
}

type Service struct {
	Name string
	// Protocol, e.g. "http".  "" for simple tcp forwarding.
	Protocol  string
	IP        net.IP
	Port      int
	Instances []Instance
}

func (svc *Service) TCPAddr() *net.TCPAddr {
	return &net.TCPAddr{IP: svc.IP, Port: svc.Port}
}

func (svc *Service) Summary() string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "%s %s/%s {", svc.Name, svc.TCPAddr(), svc.Protocol)

	comma := ""
	for _, inst := range svc.Instances {
		fmt.Fprintf(&buf, "%s%s(%s) %s", comma, inst.Name, inst.Group, inst.TCPAddr())
		comma = ", "
	}

	buf.WriteString("}")
	return buf.String()
}

func (a *Service) Equal(b *Service) bool {
	if a.Name != b.Name || a.Protocol != b.Protocol || a.Port != b.Port || !a.IP.Equal(b.IP) {
		return false
	}

	type instKey struct {
		name  string
		group string
		ip    string
		port  int
	}

	key := func(i *Instance) instKey {
		return instKey{i.Name, i.Group, string(i.IP), i.Port}
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
	Service
	Delete bool
}

type Controller interface {
	Updates() <-chan ServiceUpdate
	Close()
}
