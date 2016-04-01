package netutil

import (
	"fmt"
	"net"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/common/daemon"
)

type hostIPSlot struct {
	slot *net.IP
}

type hostIPKey struct{}

func HostIPDependency(slot *net.IP) daemon.DependencySlot {
	return hostIPSlot{slot}
}

func (hostIPSlot) Key() daemon.DependencyKey {
	return hostIPKey{}
}

func (s hostIPSlot) Assign(value interface{}) {
	*s.slot = value.(net.IP)
}

type hostIPConfig struct {
	hostIP string
}

func (hostIPKey) MakeConfig() daemon.DependencyConfig {
	return &hostIPConfig{}
}

func (cf *hostIPConfig) Populate(deps *daemon.Dependencies) {
	deps.StringVar(&cf.hostIP, "host-ip", "", "externally accessible IP address for host")
}

func (cf *hostIPConfig) MakeValue() (interface{}, error) {
	hostIP, source, err := cf.findHostIP()
	if err != nil {
		return nil, err
	}

	log.Infof("Using host IP address %s from %s", hostIP, source)
	return hostIP, nil
}

func (cf *hostIPConfig) findHostIP() (net.IP, string, error) {
	source := "-host-ip option"
	hostIP := cf.hostIP

	if hostIP == "" {
		source = "HOST_IP in environment"
		hostIP = os.Getenv("HOST_IP")
	}

	if hostIP != "" {
		ip := net.ParseIP(hostIP)
		if ip == nil {
			return nil, "", fmt.Errorf("Bad host IP address '%' from %s", hostIP, source)
		}

		return ip, source, nil
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, "", fmt.Errorf("Unable to determine host IP via hostname: %s", err)
	}

	ip, err := net.ResolveIPAddr("ip", hostname)
	if err != nil {
		return nil, "", fmt.Errorf("Unable to determine host IP via hostname: %s", err)
	}

	return ip.IP, fmt.Sprintf("hostname '%s'", hostname), nil
}
