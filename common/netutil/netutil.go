package netutil

import (
	"fmt"
	"net"
)

// Check that a string can be parsed as "ipaddress:port", and return
// the net.TCPAddr made from those parts if so.
func ParseTCPAddr(addrPort, network string, emptyAddrOk bool) (*net.TCPAddr, error) {
	ip, port, err := SplitIPAddressPort(addrPort, network, emptyAddrOk)
	if err != nil {
		return nil, err
	}
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}, nil
}

// Check that an "ipaddress:port" string looks reasonable, and split it
// into an IP address and port, resolving the port.  network is a go net
// pkg network type identifier.
func SplitIPAddressPort(addrPort string, network string, emptyAddrOk bool) (net.IP, int, error) {
	var ip net.IP
	addr, port, err := net.SplitHostPort(addrPort)
	if err != nil {
		return nil, 0, err
	}

	if addr == "" {
		if !emptyAddrOk {
			return nil, 0, fmt.Errorf("expected IP address in '%s'",
				addrPort)
		}
	} else if ip = net.ParseIP(addr); ip == nil {
		return nil, 0, fmt.Errorf("bad IP address in '%s'", addrPort)
	}

	portNum, err := net.LookupPort(network, port)
	if err != nil {
		return nil, 0, err
	}

	return ip, portNum, nil
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
