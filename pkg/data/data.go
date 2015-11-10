package data

import (
	"fmt"
	"strings"
)

type AddressSpec struct {
	Type string
	Port int
}

type InstanceSpec struct {
	AddressSpec AddressSpec       `json:"addressSpec,omitempty"`
	Selector    map[string]string `json:"selector,omitempty"`
}

type Service struct {
	Address      string       `json:"address,omitempty"`
	Port         int          `json:"port,omitempty"`
	Protocol     string       `json:"protocol,omitempty"`
	InstanceSpec InstanceSpec `json:"instanceSpec,omitempty"`
}

type Instance struct {
	Address string            `json:"address,omitempty"`
	Port    int               `json:"port,omitempty"`
	Labels  map[string]string `json:"labels"`
}

const ServicePath = "/weave/service/"

func DecodePath(path string) (serviceName, instanceName string, err error) {
	if path+"/" == ServicePath {
		return "", "", nil
	}
	part := strings.Split(path, "/")
	if len(part) < 4 {
		return "", "", fmt.Errorf("bad path: %s", path)
	} else if len(part) < 5 {
		return part[3], "", nil
	}
	return part[3], part[4], nil
}
