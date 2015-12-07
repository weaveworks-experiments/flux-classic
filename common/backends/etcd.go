package backends

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/squaremo/ambergreen/common/data"

	etcd_errors "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
)

const servicePath = "/weave/service/"

type Backend struct {
	client *etcd.Client
}

func NewBackendFromEnv() *Backend {
	etcd_address := os.Getenv("ETCD_PORT")
	if etcd_address == "" {
		etcd_address = os.Getenv("ETCD_ADDRESS")
	}
	if strings.HasPrefix(etcd_address, "tcp:") {
		etcd_address = "http:" + etcd_address[4:]
	}
	if etcd_address == "" {
		etcd_address = "http://127.0.0.1:4001"
	}

	return NewBackend(etcd_address)
}

func NewBackend(addr string) *Backend {
	return &Backend{client: etcd.NewClient([]string{addr})}
}

// Check if we can talk to etcd
func (b *Backend) Ping() error {
	rr := etcd.NewRawRequest("GET", "version", nil, nil)
	_, err := b.client.SendRequest(rr)
	return err
}

func (b *Backend) CheckRegisteredService(serviceName string) error {
	_, err := b.client.Get(servicePath+serviceName, false, false)
	return err
}

func (b *Backend) AddService(serviceName string, details data.Service) error {
	json, err := json.Marshal(&details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	_, err = b.client.Set(servicePath+serviceName+"/details", string(json), 0)
	return err
}

func (b *Backend) RemoveService(serviceName string) error {
	_, err := b.client.Delete(servicePath+serviceName, true)
	return err
}

func (b *Backend) RemoveAllServices() error {
	_, err := b.client.Delete(servicePath, true)
	return err
}

func (b *Backend) GetServiceDetails(serviceName string) (data.Service, error) {
	var service data.Service
	details, err := b.client.Get(servicePath+serviceName+"/details", false, false)
	if err != nil {
		return service, err
	}
	if err := json.Unmarshal([]byte(details.Node.Value), &service); err != nil {
		return service, err
	}
	return service, nil
}

func (b *Backend) ForeachServiceInstance(fs ServiceFunc, fi ServiceInstanceFunc) error {
	r, err := b.client.Get(servicePath, true, fi != nil)
	if err != nil {
		if etcderr, ok := err.(*etcd.EtcdError); ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound {
			return nil
		}
		return err
	}
	for _, node := range r.Node.Nodes {
		serviceName := strings.TrimPrefix(node.Key, servicePath)
		serviceData, err := b.GetServiceDetails(serviceName)
		if err != nil {
			return err
		}
		if fs != nil {
			fs(serviceName, serviceData)
		}
		if fi != nil {
			for _, instance := range node.Nodes {
				if strings.HasSuffix(instance.Key, "/details") {
					continue
				}
				var instanceData data.Instance
				if err := json.Unmarshal([]byte(instance.Value), &instanceData); err != nil {
					return err
				}
				fi(serviceName, strings.TrimPrefix(instance.Key, node.Key+"/"), instanceData)
			}
		}
	}
	return nil
}

func (b *Backend) ForeachInstance(serviceName string, fi InstanceFunc) error {
	serviceKey := servicePath + serviceName + "/"
	r, err := b.client.Get(serviceKey, true, false)
	if err != nil {
		if etcderr, ok := err.(*etcd.EtcdError); ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound {
			return nil
		}
		return err
	}
	for _, instance := range r.Node.Nodes {
		if strings.HasSuffix(instance.Key, "/details") {
			continue
		}
		var instanceData data.Instance
		if err := json.Unmarshal([]byte(instance.Value), &instanceData); err != nil {
			return err
		}
		fi(strings.TrimPrefix(instance.Key, serviceKey), instanceData)
	}
	return nil
}

func (b *Backend) AddInstance(serviceName string, instanceName string, details data.Instance) error {
	json, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	if _, err := b.client.Set(servicePath+serviceName+"/"+instanceName, string(json), 0); err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}
	return nil
}

func (b *Backend) RemoveInstance(serviceName, instanceName string) error {
	_, err := b.client.Delete(servicePath+serviceName+"/"+instanceName, true)
	return err
}

func (b *Backend) WatchServices(resCh chan<- data.ServiceChange, stopCh <-chan struct{}, withInstanceChanges bool) {
	// XXX error handling

	etcdCh := make(chan *etcd.Response, 1)
	watchStopCh := make(chan bool, 1)
	go b.client.Watch(servicePath, 0, true, etcdCh, nil)

	svcs := make(map[string]struct{})

	handleResponse := func(r *etcd.Response) {
		path := r.Node.Key
		switch r.Action {
		case "delete":
			if len(path) <= len(servicePath) {
				// All services deleted
				for name := range svcs {
					resCh <- data.ServiceChange{name, true}
				}
				svcs = make(map[string]struct{})
				return
			}

			p := strings.Split(path[len(servicePath):], "/")
			if len(p) == 1 {
				// Service deleted
				delete(svcs, p[0])
				resCh <- data.ServiceChange{p[0], true}
				return
			}

			// Instance deletion
			if withInstanceChanges {
				resCh <- data.ServiceChange{p[0], false}
			}

		case "set":
			if len(path) <= len(servicePath) {
				return
			}

			p := strings.Split(path[len(servicePath):], "/")
			svcs[p[0]] = struct{}{}
			if withInstanceChanges ||
				len(p) == 2 && p[1] == "details" {
				resCh <- data.ServiceChange{p[0], false}
			}
		}
	}

	b.ForeachServiceInstance(func(name string, svc data.Service) {
		svcs[name] = struct{}{}
	}, nil)

	go func() {
		for {
			select {
			case <-stopCh:
				watchStopCh <- true
				return

			case r := <-etcdCh:
				handleResponse(r)
			}
		}
	}()
}
