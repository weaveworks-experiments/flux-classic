package model

import (
	"fmt"
	"net"
)

type IPPort struct {
	// stringified form of the IP bytes, to be used as a map key
	ip   string
	Port int
}

func (ipport IPPort) IP() net.IP {
	return net.IP(([]byte)(ipport.ip))
}

func (ipport IPPort) TCPAddr() *net.TCPAddr {
	return &net.TCPAddr{IP: ipport.IP(), Port: ipport.Port}
}

type Ident struct {
	Individual string
	Group      string
}

type Instance struct {
	Ident
	IPPort
}

func MakeInstance(indy, group string, ip net.IP, port int) Instance {
	return Instance{Ident{indy, group}, IPPort{string(ip), port}}
}

type ServiceKey struct {
	// Type of the service, e.g. "tcp" or "udp"
	Type string
	IPPort
}

func (s ServiceKey) String() string {
	return fmt.Sprintf("%s:%s", s.Type, s.IPPort.TCPAddr().String())
}

func MakeServiceKey(typ string, ip net.IP, port int) ServiceKey {
	return ServiceKey{typ, IPPort{string(ip), port}}
}

type ServiceInfo struct {
	// Protocol, e.g. "http".  "" for simple tcp forwarding.
	Protocol  string
	Instances []Instance
}

type ServiceUpdate struct {
	ServiceKey
	*ServiceInfo
}
