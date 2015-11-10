package main

import (
	"flag"
	"log"
	"net"
	"os"

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
	var (
		hostIP string
	)
	flag.StringVar(&hostIP, "host-ip", "", "IP address for instances with mapped ports")
	flag.Parse()

	dc, err := setupDockerClient()
	if err != nil {
		log.Fatal("Error connecting to docker: ", err)
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
	}

	listener := NewListener(Config{
		HostIP: hostIP,
	}, dc)

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
