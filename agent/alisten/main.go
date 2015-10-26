// Integrate coatl with ambergris
package main

import (
	"bytes"
	"fmt"
	"log"
	"net"

	"github.com/bboreham/coatl/backends"
	"github.com/bboreham/coatl/data"
)

var backend *backends.Backend

func main() {
	backend = backends.NewBackend([]string{})
	run()
}

const SOCKET = "/var/run/ambergris.sock"

func sendToAmber(serviceName string) error {
	service, err := backend.GetServiceDetails(serviceName)
	if err != nil {
		return err
	}
	var instances []data.Instance
	backend.ForeachInstance(serviceName, func(name string, instance data.Instance) {
		instances = append(instances, instance)
	})
	conn, err := net.Dial("unix", SOCKET)
	if err != nil {
		log.Println("Failed to connect to ambergris socket:", err)
		return err
	}
	buf := new(bytes.Buffer)
	buf.WriteString(fmt.Sprintf("%s:%d", service.Address, service.Port))
	for _, instance := range instances {
		buf.WriteString(fmt.Sprintf(" %s:%d", instance.Address, instance.Port))
	}
	log.Println("Sending to ambergris:", buf.String())
	_, err = conn.Write(buf.Bytes())
	return err
}

func run() {
	ch := backend.Watch()

	for r := range ch {
		//fmt.Println(r.Action, r.Node)
		serviceName, _, err := data.DecodePath(r.Node.Key)
		if err != nil {
			log.Println(err)
			continue
		}
		if serviceName == "" {
			// everything deleted; can't cope
			continue
		}
		sendToAmber(serviceName)
	}
}
