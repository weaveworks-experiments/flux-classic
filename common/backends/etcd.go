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

	return &Backend{client: etcd.NewClient([]string{etcd_address})}
}

// Check if we can talk to etcd
func (b *Backend) Ping() error {
	rr := etcd.NewRawRequest("GET", "version", nil, nil)
	_, err := b.client.SendRequest(rr)
	return err
}

func (b *Backend) CheckRegisteredService(serviceName string) error {
	_, err := b.client.Get(data.ServicePath+serviceName, false, false)
	return err
}

func (b *Backend) AddService(serviceName string, details data.Service) error {
	json, err := json.Marshal(&details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}
	_, err = b.client.Set(data.ServicePath+serviceName+"/details", string(json), 0)
	return err
}

func (b *Backend) RemoveService(serviceName string) error {
	_, err := b.client.Delete(data.ServicePath+serviceName, true)
	return err
}

func (b *Backend) RemoveAllServices() error {
	_, err := b.client.Delete(data.ServicePath, true)
	return err
}

func (b *Backend) GetServiceDetails(serviceName string) (data.Service, error) {
	var service data.Service
	details, err := b.client.Get(data.ServicePath+serviceName+"/details", false, false)
	if err != nil {
		return service, err
	}
	if err := json.Unmarshal([]byte(details.Node.Value), &service); err != nil {
		return service, err
	}
	return service, nil
}

func (b *Backend) ForeachServiceInstance(fs func(string, data.Service), fi func(string, data.Instance)) error {
	r, err := b.client.Get(data.ServicePath, true, fi != nil)
	if err != nil {
		if etcderr, ok := err.(*etcd.EtcdError); ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound {
			return nil
		}
		return err
	}
	for _, node := range r.Node.Nodes {
		serviceName := strings.TrimPrefix(node.Key, data.ServicePath)
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
				fi(strings.TrimPrefix(instance.Key, node.Key+"/"), instanceData)
			}
		}
	}
	return nil
}

func (b *Backend) ForeachInstance(serviceName string, fi func(string, data.Instance)) error {
	serviceKey := data.ServicePath + serviceName + "/"
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
	if _, err := b.client.Set(data.ServicePath+serviceName+"/"+instanceName, string(json), 0); err != nil {
		return fmt.Errorf("Unable to write: %s", err)
	}
	return nil
}

func (b *Backend) RemoveInstance(serviceName, instanceName string) error {
	_, err := b.client.Delete(data.ServicePath+serviceName+"/"+instanceName, true)
	return err
}

// Needs work to make less etcd-centric
func (b *Backend) Watch() chan *etcd.Response {
	ch := make(chan *etcd.Response)
	go b.client.Watch(data.ServicePath, 0, true, ch, nil)
	return ch
}
