package netutil

import (
	"fmt"
	"net"
	"strconv"
)

// IPPort is opaque, to allow a representation that works as a map key
type IPPort struct {
	ip   string
	port int
}

func NewIPPort(ip net.IP, port int) IPPort {
	return IPPort{string(ip), port}
}

func (ipPort IPPort) IP() net.IP {
	if len(ipPort.ip) == 0 {
		return nil
	} else {
		return net.IP(ipPort.ip)
	}
}

func (ipPort IPPort) Port() int {
	return ipPort.port
}

func (ipPort IPPort) String() string {
	var ipStr string
	if len(ipPort.ip) != 0 {
		ip := net.IP(ipPort.ip)
		if !ip.IsUnspecified() {
			ipStr = ip.String()
		}
	}

	return net.JoinHostPort(ipStr, strconv.Itoa(ipPort.port))
}

func (ipPort *IPPort) TCPAddr() *net.TCPAddr {
	if ipPort == nil {
		return nil
	} else {
		return &net.TCPAddr{IP: ipPort.IP(), Port: ipPort.Port()}
	}
}

func (a IPPort) Equal(b IPPort) bool {
	return a == b
}

func (a IPPort) LessThan(b IPPort) bool {
	switch {
	case a.ip < b.ip:
		return true
	case a.ip == b.ip:
		return a.port < b.port
	default:
		return false
	}
}

// Check that a string can be parsed as "ipaddress:port", and return
// the IPPort made from those parts if so.
func ParseIPPort(addrPort string) (IPPort, error) {
	var ip net.IP
	addr, port, err := net.SplitHostPort(addrPort)
	if err != nil {
		return IPPort{}, err
	}

	if addr != "" {
		if ip = net.ParseIP(addr); ip == nil {
			return IPPort{}, fmt.Errorf("bad IP address in '%s'", addrPort)
		}
	}

	portNum, err := net.LookupPort("", port)
	if err != nil {
		return IPPort{}, err
	}

	return NewIPPort(ip, portNum), err
}

// For use in testing
func ParseIPPortPtr(addrPort string) *IPPort {
	addr, _ := ParseIPPort(addrPort)
	return &addr
}

func (ipPort IPPort) MarshalText() ([]byte, error) {
	return ([]byte)(ipPort.String()), nil
}

func (ipPort *IPPort) UnmarshalText(text []byte) error {
	var err error
	*ipPort, err = ParseIPPort(string(text))
	return err
}

// Check that a "host:port" string looks reasonable, and split it
// into and host and port, resolving the port.  network is a go net
// pkg network type identifier.
func SplitHostPort(hostPort string, network string, emptyHostOk bool) (string, int, error) {
	host, port, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", 0, err
	}

	if host == "" {
		if !emptyHostOk {
			return "", 0, fmt.Errorf("expected hostname in '%s'",
				hostPort)
		}
	} else if host[0] == ':' || (host[0] >= '0' && host[0] <= '9') {
		// host looks like an IP address, validate it
		if net.ParseIP(host) == nil {
			return "", 0, fmt.Errorf("bad IP address in '%s'", hostPort)
		}
	}

	portNum, err := net.LookupPort(network, port)
	if err != nil {
		return "", 0, err
	}

	return host, portNum, nil
}

// Check that a "host:port" string looks reasonable, and resolve the
// port.  network is a go net pkg network type identifier.
func NormalizeHostPort(hostPort string, network string, emptyHostOk bool) (string, error) {
	host, portNum, err := SplitHostPort(hostPort, network, emptyHostOk)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%d", host, portNum), nil
}
