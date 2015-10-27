package main

import (
	"log"

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
	log.Printf("Using Docker %v", env)
	return dc, nil
}

func main() {
	dc, err := setupDockerClient()
	if err != nil {
		log.Fatal("Error connecting to docker: ", err)
	}
	listener := NewListener(dc)

	events := make(chan *docker.APIEvents)
	if err := dc.AddEventListener(events); err != nil {
		log.Fatalf("Unable to add listener to Docker API: %s", err)
	}

	if err := listener.ReadInServices(); err != nil {
		log.Fatal("Error reading configuration: ", err)
	}
	if err := listener.ReadExistingContainers(); err != nil {
		log.Fatal("Error reading existing containers:", err)
	}
	if err := listener.Sync(); err != nil {
		log.Fatal("Error synchronising existing containers:", err)
	}
	listener.Run(events)
}
