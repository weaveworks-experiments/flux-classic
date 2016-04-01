package agent

import (
	"fmt"
	"net"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/heartbeat"
	"github.com/weaveworks/flux/common/netutil"
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
	hostIP            net.IP
	network           string
	store             store.Store
	dockerClient      DockerClient
	reconnectInterval time.Duration
}

func (cf *AgentConfig) Populate(deps *daemon.Dependencies) {
	deps.IntVar(&cf.hostTTL, "host-ttl", 30, "Time-to-live for host record; the daemon will try to refresh this on a schedule such that it doesn't lapse")
	deps.StringVar(&cf.network, "network-mode", LOCAL, fmt.Sprintf(`Kind of network to assume for containers (either "%s" or "%s")`, LOCAL, GLOBAL))
	deps.Dependency(etcdstore.StoreDependency(&cf.store))
	deps.Dependency(netutil.HostIPDependency(&cf.hostIP))
}

func (cf *AgentConfig) Prepare() (daemon.StartFunc, error) {
	if !IsValidNetworkMode(cf.network) {
		return nil, fmt.Errorf("Unknown network mode '%s'", cf.network)
	}

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
		HostIdentity: cf.hostIP.String(),
		HostState:    &data.Host{IPAddress: cf.hostIP.String()},
	}

	containerUpdates := make(chan ContainerUpdate)
	containerUpdatesReset := make(chan struct{})
	serviceUpdates := make(chan store.ServiceUpdate)
	serviceUpdatesReset := make(chan struct{})

	siconf := syncInstancesConfig{
		hostIP:  cf.hostIP.String(),
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
