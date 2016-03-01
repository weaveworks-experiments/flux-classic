package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/common/store/etcdstore"
	"github.com/weaveworks/flux/common/version"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
)

func setupDockerClient() (*docker.Client, error) {
	dc, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	env, err := dc.Version()
	if err != nil {
		return nil, err
	}
	log.Infof("Using Docker %+v", env)
	return dc, nil
}

func main() {
	var (
		hostIP  string
		network string
	)
	flag.StringVar(&hostIP, "host-ip", "", "IP address for instances with mapped ports")
	flag.StringVar(&network, "network-mode", agent.LOCAL, fmt.Sprintf(`Kind of network to assume for containers (either "%s" or "%s")`, agent.LOCAL, agent.GLOBAL))
	flag.Parse()

	if !agent.IsValidNetworkMode(network) {
		fmt.Fprintf(os.Stderr, "Unknown network mode \"%s\"\n\n", network)
		flag.Usage()
		os.Exit(1)
	}

	log.Infof("flux agent version %s", version.Version())

	dc, err := setupDockerClient()
	if err != nil {
		log.Fatalf("Error connecting to docker: %s", err)
	}

	hostIpFrom := "argument"

	if hostIP == "" {
		hostIP = os.Getenv("HOST_IP")
		hostIpFrom = `$HOST_IP in environment`
	}

	if hostIP == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("Unable to determine host IP via hostname: %s", err)
		}
		ip, err := net.ResolveIPAddr("ip", hostname)
		if err != nil {
			log.Fatalf("Unable to determine host IP via hostname: %s", err)
		}
		hostIP = ip.String()
		hostIpFrom = fmt.Sprintf(`resolving hostname '%s'`, hostname)
	}

	log.Infof(`Using host IP address '%s' from %s`, hostIP, hostIpFrom)

	store, err := etcdstore.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	si := agent.NewSyncInstances(agent.Config{
		HostIP:    hostIP,
		Network:   network,
		Store:     store,
		Inspector: dc,
	})

	events := make(chan *docker.APIEvents)
	if err := dc.AddEventListener(events); err != nil {
		log.Fatalf("Unable to add listener to Docker API: %s", err)
	}

	if err := si.ReadExistingContainers(); err != nil {
		log.Fatalf("Error reading existing containers: %s", err)
	}
	if err := si.ReadInServices(); err != nil {
		log.Fatalf("Error reading configuration: %s", err)
	}
	si.Run(events)
}
