package agent

import (
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
)

type DockerClient interface {
	Version() (*docker.Env, error)
	AddEventListener(listener chan<- *docker.APIEvents) error
	RemoveEventListener(listener chan *docker.APIEvents) error
	ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error)
	InspectContainer(id string) (*docker.Container, error)
}

type AgentConfig struct {
	HostIP          string
	Network         string
	Store           store.Store
	DockerClient    DockerClient
	RestartInterval time.Duration
}

func (conf AgentConfig) StartFunc() daemon.StartFunc {
	containerUpdates := make(chan ContainerUpdate)
	containerUpdatesReset := make(chan struct{})
	serviceUpdates := make(chan store.ServiceUpdate)
	serviceUpdatesReset := make(chan struct{})

	siconf := syncInstancesConfig{
		hostIP:  conf.HostIP,
		network: conf.Network,
		store:   conf.Store,

		containerUpdates:      containerUpdates,
		containerUpdatesReset: containerUpdatesReset,
		serviceUpdates:        serviceUpdates,
		serviceUpdatesReset:   serviceUpdatesReset,
	}

	return daemon.Aggregate(
		daemon.Reset(containerUpdatesReset,
			daemon.Restart(conf.RestartInterval,
				dockerListenerStartFunc(conf.DockerClient,
					containerUpdates))),
		daemon.Reset(serviceUpdatesReset,
			daemon.Restart(conf.RestartInterval,
				store.WatchServicesStartFunc(conf.Store,
					store.QueryServiceOptions{WithContainerRules: true},
					serviceUpdates))),
		daemon.Restart(conf.RestartInterval, siconf.StartFunc()))
}
