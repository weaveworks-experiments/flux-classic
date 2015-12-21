package model

import (
	"net"
)

type Instance struct {
	Name  string
	Group string
	IP    net.IP
	Port  int
}

func (inst *Instance) TCPAddr() net.TCPAddr {
	return net.TCPAddr{IP: inst.IP, Port: inst.Port}
}

type Service struct {
	Name string
	// Protocol, e.g. "http".  "" for simple tcp forwarding.
	Protocol  string
	IP        net.IP
	Port      int
	Instances []Instance
}

func (svc *Service) TCPAddr() net.TCPAddr {
	return net.TCPAddr{IP: svc.IP, Port: svc.Port}
}

type ServiceUpdate struct {
	Service
	Delete bool
}
