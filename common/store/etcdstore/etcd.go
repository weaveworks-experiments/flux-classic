package etcdstore

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	etcd_errors "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/errorsink"
	"github.com/squaremo/ambergreen/common/store"
)

type etcdStore struct {
	client *etcd.Client
}

func NewFromEnv() store.Store {
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

	return New(etcd_address)
}

func New(addr string) store.Store {
	return &etcdStore{client: etcd.NewClient([]string{addr})}
}

// Check if we can talk to etcd
func (es *etcdStore) Ping() error {
	rr := etcd.NewRawRequest("GET", "version", nil, nil)
	_, err := es.client.SendRequest(rr)
	return err
}

const ROOT = "/weave/service/"

func serviceRootKey(serviceName string) string {
	return ROOT + serviceName
}

func serviceKey(serviceName string) string {
	return fmt.Sprintf("%s%s/details", ROOT, serviceName)
}

func groupSpecKey(serviceName, groupName string) string {
	return fmt.Sprintf("%s%s/groupspec/%s", ROOT, serviceName, groupName)
}

func instanceKey(serviceName, instanceName string) string {
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

type parsedGroupSpecKey struct {
	serviceName string
	groupName   string
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

	case "groupspec":
		if len(p) == 3 {
			return parsedGroupSpecKey{p[0], p[2]}
		}

	case "instance":
		if len(p) == 3 {
			return parsedInstanceKey{p[0], p[2]}
		}
	}

	return nil
}

func (es *etcdStore) CheckRegisteredService(serviceName string) error {
	_, err := es.client.Get(serviceRootKey(serviceName), false, false)
	return err
}

func (es *etcdStore) AddService(serviceName string, details data.Service) error {
	json, err := json.Marshal(&details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	_, err = es.client.Set(serviceKey(serviceName), string(json), 0)
	return err
}

func (es *etcdStore) RemoveService(serviceName string) error {
	_, err := es.client.Delete(serviceRootKey(serviceName), true)
	return err
}

func (es *etcdStore) RemoveAllServices() error {
	_, err := es.client.Delete(ROOT, true)
	return err
}

func (es *etcdStore) GetServiceDetails(serviceName string) (data.Service, error) {
	r, err := es.client.Get(serviceKey(serviceName), false, false)
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

func unmarshalGroupSpec(node *etcd.Node) (data.InstanceGroupSpec, error) {
	var gs data.InstanceGroupSpec
	err := json.Unmarshal([]byte(node.Value), &gs)
	return gs, err
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

func (es *etcdStore) traverse(key string, f func(*etcd.Node) error) error {
	r, err := es.client.Get(key, false, true)
	if err != nil {
		if etcderr, ok := err.(*etcd.EtcdError); ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound {
			return nil
		}
		return err
	}

	return traverse(r.Node, f)
}

func (es *etcdStore) ForeachServiceInstance(fs store.ServiceFunc, fi store.ServiceInstanceFunc) error {
	return es.traverse(ROOT, func(node *etcd.Node) error {
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

func (es *etcdStore) GetInstanceGroupSpecs(serviceName string) (map[string]data.InstanceGroupSpec, error) {
	res := make(map[string]data.InstanceGroupSpec)
	err := es.traverse(serviceRootKey(serviceName), func(node *etcd.Node) error {
		switch key := parseKey(node.Key).(type) {
		case parsedGroupSpecKey:
			gs, err := unmarshalGroupSpec(node)
			if err != nil {
				return err
			}

			res[key.groupName] = gs
		}

		return nil
	})

	return res, err
}

func (es *etcdStore) SetInstanceGroupSpec(serviceName string, groupName string, spec data.InstanceGroupSpec) error {
	json, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	if _, err := es.client.Set(groupSpecKey(serviceName, groupName), string(json), 0); err != nil {
		return err
	}

	return err
}

func (es *etcdStore) RemoveInstanceGroupSpec(serviceName string, groupName string) error {
	_, err := es.client.Delete(groupSpecKey(serviceName, groupName), true)
	return err
}

func (es *etcdStore) AddInstance(serviceName string, instanceName string, details data.Instance) error {
	json, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	if _, err := es.client.Set(instanceKey(serviceName, instanceName), string(json), 0); err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}
	return nil
}

func (es *etcdStore) RemoveInstance(serviceName, instanceName string) error {
	_, err := es.client.Delete(instanceKey(serviceName, instanceName), true)
	return err
}

func (es *etcdStore) ForeachInstance(serviceName string, fi store.InstanceFunc) error {
	return es.traverse(serviceRootKey(serviceName), func(node *etcd.Node) error {
		switch key := parseKey(node.Key).(type) {
		case parsedInstanceKey:
			inst, err := unmarshalInstance(node)
			if err != nil {
				return err
			}

			fi(key.instanceName, inst)
		}

		return nil
	})
}

func (es *etcdStore) WatchServices(resCh chan<- data.ServiceChange, stopCh <-chan struct{}, errorSink errorsink.ErrorSink, opts store.WatchServicesOptions) {
	etcdCh := make(chan *etcd.Response, 1)
	watchStopCh := make(chan bool, 1)
	go func() {
		_, err := es.client.Watch(ROOT, 0, true, etcdCh, nil)
		if err != nil {
			errorSink.Post(err)
		}
	}()

	svcs := make(map[string]struct{})
	es.ForeachServiceInstance(func(name string, svc data.Service) {
		svcs[name] = struct{}{}
	}, nil)

	handleResponse := func(r *etcd.Response) {
		// r is nil on error
		if r == nil {
			return
		}

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
				if opts.WithInstanceChanges {
					resCh <- data.ServiceChange{key.serviceName, false}
				}
			}

		case "set":
			switch key := parseKey(r.Node.Key).(type) {
			case parsedServiceKey:
				svcs[key.serviceName] = struct{}{}
				resCh <- data.ServiceChange{key.serviceName, false}

			case parsedInstanceKey:
				if opts.WithInstanceChanges {
					resCh <- data.ServiceChange{key.serviceName, false}
				}
			}
		}
	}

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
