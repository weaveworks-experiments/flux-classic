package agent

import (
	"fmt"
	"net"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/heartbeat"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
)

type DockerClient interface {
	Version() (*docker.Env, error)
	AddEventListener(listener chan<- *docker.APIEvents) error
	RemoveEventListener(listener chan *docker.APIEvents) error
	ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error)
	InspectContainer(id string) (*docker.Container, error)
}

type AgentConfig struct {
	hostTTL           int
	hostIP            string
	network           string
	store             store.Store
	dockerClient      DockerClient
	reconnectInterval time.Duration
}

func (cf *AgentConfig) Populate(deps *daemon.Dependencies) {
	deps.IntVar(&cf.hostTTL, "host-ttl", 30, "Time-to-live for host record; the agent will try to refresh this on a schedule such that it doesn't lapse")
	deps.StringVar(&cf.hostIP, "host-ip", "", "IP address for instances with mapped ports")
	deps.StringVar(&cf.network, "network-mode", LOCAL, fmt.Sprintf(`Kind of network to assume for containers (either "%s" or "%s")`, LOCAL, GLOBAL))
	deps.Dependency(etcdstore.StoreDependency(&cf.store))
}

func (cf *AgentConfig) Prepare() (daemon.StartFunc, error) {
	if !IsValidNetworkMode(cf.network) {
		return nil, fmt.Errorf("Unknown network mode '%s'", cf.network)
	}

	hostIpFrom := "argument"

	if cf.hostIP == "" {
		cf.hostIP = os.Getenv("HOST_IP")
		hostIpFrom = "HOST_IP in environment"
	}

	if cf.hostIP == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("Unable to determine host IP via hostname: %s", err)
		}
		ip, err := net.ResolveIPAddr("ip", hostname)
		if err != nil {
			return nil, fmt.Errorf("Unable to determine host IP via hostname: %s", err)
		}
		cf.hostIP = ip.String()
		hostIpFrom = fmt.Sprintf("resolving hostname '%s'", hostname)
	}

	log.Infof("Using host IP address '%s' from %s", cf.hostIP, hostIpFrom)

	if cf.dockerClient == nil {
		var err error
		if cf.dockerClient, err = docker.NewClientFromEnv(); err != nil {
			return nil, err
		}
	}

	if cf.reconnectInterval == 0 {
		cf.reconnectInterval = 10 * time.Second
	}

	hb := heartbeat.HeartbeatConfig{
		Cluster:      cf.store,
		TTL:          time.Duration(cf.hostTTL) * time.Second,
		HostIdentity: cf.hostIP,
		HostState:    &data.Host{IPAddress: cf.hostIP},
	}

	containerUpdates := make(chan ContainerUpdate)
	containerUpdatesReset := make(chan struct{})
	serviceUpdates := make(chan store.ServiceUpdate)
	serviceUpdatesReset := make(chan struct{})

	siconf := syncInstancesConfig{
		hostIP:  cf.hostIP,
		network: cf.network,
		store:   cf.store,

		containerUpdates:      containerUpdates,
		containerUpdatesReset: containerUpdatesReset,
		serviceUpdates:        serviceUpdates,
		serviceUpdatesReset:   serviceUpdatesReset,
	}

	return daemon.Aggregate(
		daemon.Reset(containerUpdatesReset,
			daemon.Restart(cf.reconnectInterval,
				dockerListenerStartFunc(cf.dockerClient,
					containerUpdates))),

		daemon.Reset(serviceUpdatesReset,
			daemon.Restart(cf.reconnectInterval,
				store.WatchServicesStartFunc(cf.store,
					store.QueryServiceOptions{WithContainerRules: true},
					serviceUpdates))),

		daemon.Restart(cf.reconnectInterval, siconf.StartFunc()),

		daemon.Restart(cf.reconnectInterval, hb.Start)), nil
}
