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

const ROOT = "/weave/service/"

func serviceRootKey(serviceName string) string {
	return ROOT + serviceName
}

func serviceKey(serviceName string) string {
	return fmt.Sprintf("%s%s/details", ROOT, serviceName)
}

func instanceKey(serviceName string, instanceName string) string {
	return fmt.Sprintf("%s%s/instance/%s", ROOT, serviceName, instanceName)
}

type parsedRootKey struct {
}

type parsedServiceRootKey struct {
	serviceName string
}

type parsedServiceKey struct {
	serviceName string
}

type parsedInstanceKey struct {
	serviceName  string
	instanceName string
}

// Parse a path to find its type

func parseKey(key string) interface{} {
	if len(key) <= len(ROOT) {
		return parsedRootKey{}
	}

	p := strings.Split(key[len(ROOT):], "/")
	if len(p) == 1 {
		return parsedServiceRootKey{p[0]}
	}

	switch p[1] {
	case "details":
		return parsedServiceKey{p[0]}

	case "instance":
		if len(p) == 3 {
			return parsedInstanceKey{p[0], p[2]}
		}
	}

	return nil
}

func (b *Backend) CheckRegisteredService(serviceName string) error {
	_, err := b.client.Get(serviceRootKey(serviceName), false, false)
	return err
}

func (b *Backend) AddService(serviceName string, details data.Service) error {
	json, err := json.Marshal(&details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	_, err = b.client.Set(serviceKey(serviceName), string(json), 0)
	return err
}

func (b *Backend) RemoveService(serviceName string) error {
	_, err := b.client.Delete(serviceRootKey(serviceName), true)
	return err
}

func (b *Backend) RemoveAllServices() error {
	_, err := b.client.Delete(ROOT, true)
	return err
}

func (b *Backend) GetServiceDetails(serviceName string) (data.Service, error) {
	r, err := b.client.Get(serviceKey(serviceName), false, false)
	if err != nil {
		return data.Service{}, err
	}

	return unmarshalService(r.Node)
}

func unmarshalService(node *etcd.Node) (data.Service, error) {
	var service data.Service
	err := json.Unmarshal([]byte(node.Value), &service)
	return service, err
}

func unmarshalInstance(node *etcd.Node) (data.Instance, error) {
	var instance data.Instance
	err := json.Unmarshal([]byte(node.Value), &instance)
	return instance, err
}

func traverse(node *etcd.Node, f func(*etcd.Node) error) error {
	for _, child := range node.Nodes {
		var err error
		if child.Dir {
			err = traverse(child, f)
		} else {
			err = f(child)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Backend) traverse(key string, f func(*etcd.Node) error) error {
	r, err := b.client.Get(key, false, true)
	if err != nil {
		if etcderr, ok := err.(*etcd.EtcdError); ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound {
			return nil
		}
		return err
	}

	return traverse(r.Node, f)
}

func (b *Backend) ForeachServiceInstance(fs ServiceFunc, fi ServiceInstanceFunc) error {
	return b.traverse(ROOT, func(node *etcd.Node) error {
		switch key := parseKey(node.Key).(type) {
		case parsedServiceKey:
			if fs != nil {
				svc, err := unmarshalService(node)
				if err != nil {
					return err
				}

				fs(key.serviceName, svc)
			}

		case parsedInstanceKey:
			if fi != nil {
				inst, err := unmarshalInstance(node)
				if err != nil {
					return err
				}

				fi(key.serviceName, key.instanceName, inst)
			}
		}

		return nil
	})
}

func (b *Backend) AddInstance(serviceName string, instanceName string, details data.Instance) error {
	json, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	if _, err := b.client.Set(instanceKey(serviceName, instanceName), string(json), 0); err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}
	return nil
}

func (b *Backend) RemoveInstance(serviceName, instanceName string) error {
	_, err := b.client.Delete(instanceKey(serviceName, instanceName), true)
	return err
}

func (b *Backend) ForeachInstance(serviceName string, fi InstanceFunc) error {
	return b.traverse(serviceRootKey(serviceName), func(node *etcd.Node) error {
		switch key := parseKey(node.Key).(type) {
		case parsedInstanceKey:
			if fi != nil {
				inst, err := unmarshalInstance(node)
				if err != nil {
					return err
				}

				fi(key.instanceName, inst)
			}
		}

		return nil
	})
}

func (b *Backend) WatchServices(resCh chan<- data.ServiceChange, stopCh <-chan struct{}, withInstanceChanges bool) {
	// XXX error handling

	etcdCh := make(chan *etcd.Response, 1)
	watchStopCh := make(chan bool, 1)
	go b.client.Watch(ROOT, 0, true, etcdCh, nil)

	svcs := make(map[string]struct{})

	handleResponse := func(r *etcd.Response) {
		switch r.Action {
		case "delete":
			switch key := parseKey(r.Node.Key).(type) {
			case parsedRootKey:
				for name := range svcs {
					resCh <- data.ServiceChange{name, true}
				}
				svcs = make(map[string]struct{})

			case parsedServiceRootKey:
				delete(svcs, key.serviceName)
				resCh <- data.ServiceChange{key.serviceName, true}

			case parsedInstanceKey:
				if withInstanceChanges {
					resCh <- data.ServiceChange{key.serviceName, false}
				}
			}

		case "set":
			switch key := parseKey(r.Node.Key).(type) {
			case parsedServiceKey:
				svcs[key.serviceName] = struct{}{}
				resCh <- data.ServiceChange{key.serviceName, false}

			case parsedInstanceKey:
				if withInstanceChanges {
					resCh <- data.ServiceChange{key.serviceName, false}
				}
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
