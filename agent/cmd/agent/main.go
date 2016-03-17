package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/heartbeat"
	"github.com/weaveworks/flux/common/store/etcdstore"
	"github.com/weaveworks/flux/common/version"
)

const (
	DefaultHostTTL = 30
)

func main() {
	var (
		hostTTL int
		hostIP  string
		network string
	)
	flag.IntVar(&hostTTL, "host-ttl", DefaultHostTTL, "Time-to-live for host record; the agent will try to refresh this on a schedule such that it doesn't lapse")
	flag.StringVar(&hostIP, "host-ip", "", "IP address for instances with mapped ports")
	flag.StringVar(&network, "network-mode", agent.LOCAL, fmt.Sprintf(`Kind of network to assume for containers (either "%s" or "%s")`, agent.LOCAL, agent.GLOBAL))
	flag.Parse()

	if !agent.IsValidNetworkMode(network) {
		fmt.Fprintf(os.Stderr, "Unknown network mode \"%s\"\n\n", network)
		flag.Usage()
		os.Exit(1)
	}

	log.Infof("flux agent version %s", version.Version())

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

	st, err := etcdstore.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	dc, err := docker.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	hb := heartbeat.HeartbeatConfig{
		Cluster:      st,
		TTL:          time.Duration(hostTTL) * time.Second,
		HostIdentity: hostIP,
		HostState:    &data.Host{IPAddress: hostIP},
	}

	daemon.Main(daemon.Aggregate(
		agent.AgentConfig{
			HostIP:          hostIP,
			Network:         network,
			Store:           st,
			DockerClient:    dc,
			RestartInterval: 10 * time.Second,
		}.StartFunc(),
		daemon.Restart(10*time.Second, hb.Start)))
}
