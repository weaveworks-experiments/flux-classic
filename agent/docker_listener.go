package agent

import (
	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"

	"github.com/weaveworks/flux/common/daemon"
)

type ContainerUpdate struct {
	Containers map[string]*docker.Container
	Reset      bool
}

type dockerListener struct {
	client    dockerClient
	stop      <-chan struct{}
	errorSink daemon.ErrorSink
}

type dockerClient interface {
	AddEventListener(listener chan<- *docker.APIEvents) error
	RemoveEventListener(listener chan *docker.APIEvents) error
	ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error)
	InspectContainer(id string) (*docker.Container, error)
}

// Passed from the readContainerIDs goroutine to the inspectContainers
// goroutine, via the buffer goroutine
type containerIDs struct {
	containers map[string]bool
	reset      bool
}

func DockerListenerStartFunc(out chan<- ContainerUpdate) daemon.StartFunc {
	return daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		dl := dockerListener{
			stop:      stop,
			errorSink: errs,
		}
		errs.Post(dl.start(out))
	})
}

func (dl *dockerListener) start(out chan<- ContainerUpdate) error {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return err
	}

	env, err := client.Version()
	if err != nil {
		return err
	}
	log.Infof("Using Docker %+v", env)

	return dl.startAux(out)
}

func (dl *dockerListener) startAux(out chan<- ContainerUpdate) error {
	bufContainerIDs := make(chan containerIDs)
	containerIDs := make(chan containerIDs)

	daemon.Par(func() {
		dl.errorSink.Post(dl.readContainerIDs(bufContainerIDs))
	}, func() {
		// Buffer the container ID updates, because if inspecting a
		// container takes a long time, it shouldn't block handling of
		// events from the docker client.
		dl.bufferContainerIDs(bufContainerIDs, containerIDs)
	}, func() {
		dl.errorSink.Post(dl.inspectContainers(containerIDs, out))
	})

	return nil
}

func (dl *dockerListener) readContainerIDs(out chan<- containerIDs) error {
	// We need to start listening for events before we list the
	// existing containers
	events := make(chan *docker.APIEvents)
	if err := dl.client.AddEventListener(events); err != nil {
		return err
	}

	defer dl.client.RemoveEventListener(events)

	// Asynchronously retrieve the list of containers
	var listContainersRes []docker.APIContainers
	listContainersCh := make(chan error, 1)
	go func() {
		defer close(listContainersCh)
		var err error
		listContainersRes, err = dl.client.ListContainers(docker.ListContainersOptions{})
		listContainersCh <- err
	}()

	containers := make(map[string]bool)
whileListingContainers:
	for {
		select {
		case err := <-listContainersCh:
			if err != nil {
				return err
			}

			break whileListingContainers

		case ev := <-events:
			switch ev.Status {
			case "start":
				containers[ev.ID] = true
			case "die":
				containers[ev.ID] = false
			}

		case <-dl.stop:
			return nil
		}

	}

	for _, c := range listContainersRes {
		if _, present := containers[c.ID]; !present {
			containers[c.ID] = true
		}
	}

	// Remove the IDs of any containers that have died.  We do
	// this even is they were in the results of ListContainers,
	// because the "die" event must mean they have gone.
	for id, started := range containers {
		if !started {
			delete(containers, id)
		}
	}

	select {
	case out <- containerIDs{containers: containers, reset: true}:
	case <-dl.stop:
		return nil
	}

	// Handle further events
	for {
		var ev *docker.APIEvents
		select {
		case ev = <-events:
		case <-dl.stop:
			return nil
		}

		var started bool
		switch ev.Status {
		case "start":
			started = true
		case "die":
			started = false
		default:
			continue
		}

		select {
		case out <- containerIDs{
			containers: map[string]bool{ev.ID: started},
		}:
		case <-dl.stop:
			return nil
		}

	}
}

func (dl *dockerListener) bufferContainerIDs(in <-chan containerIDs, out chan<- containerIDs) {
	var buf []containerIDs

	for {
		if len(buf) == 0 {
			select {
			case ids := <-in:
				buf = append(buf, ids)
			case <-dl.stop:
				return
			}
		} else {
			select {
			case ids := <-in:
				buf = append(buf, ids)
			case out <- buf[0]:
				buf = buf[1:]
			case <-dl.stop:
				return
			}
		}
	}
}

func (dl *dockerListener) inspectContainers(in <-chan containerIDs, out chan<- ContainerUpdate) error {
	// It's possible that we'll hear about a container being
	// started, but by the time we inspect it, it's already gone.
	// In such a case, we want to suppress both the arrival and
	// the disappearance of the container.  So we have to keep
	// track of which containers we have announced.
	announced := make(map[string]struct{})

	for {
		var inUpdate containerIDs
		select {
		case inUpdate = <-in:
		case <-dl.stop:
			return nil
		}

		if inUpdate.reset {
			announced = make(map[string]struct{})
		}

		outUpdate := make(map[string]*docker.Container)
		for id, started := range inUpdate.containers {
			if !started {
				if _, present := announced[id]; present {
					outUpdate[id] = nil
					delete(announced, id)
				}

				continue
			}

			details, err := dl.client.InspectContainer(id)
			if err != nil {
				if _, noSuch := err.(*docker.NoSuchContainer); noSuch {
					continue
				}

				return err
			}

			outUpdate[id] = details
			announced[id] = struct{}{}
		}

		if !inUpdate.reset && len(outUpdate) == 0 {
			continue
		}

		select {
		case out <- ContainerUpdate{
			Containers: outUpdate,
			Reset:      inUpdate.reset,
		}:
		case <-dl.stop:
			return nil
		}
	}
}
