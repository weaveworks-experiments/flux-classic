package etcdstore

import (
	"encoding/json"
	"fmt"
	"strings"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/store"
)

type etcdStore struct {
	etcdutil.Client
	ctx context.Context
}

func NewFromEnv() (store.Store, error) {
	c, err := etcdutil.NewClientFromEnv()
	if err != nil {
		return nil, err
	}

	return newEtcdStore(c), nil
}

func New(c etcdutil.Client) store.Store {
	return newEtcdStore(c)
}

func newEtcdStore(c etcdutil.Client) *etcdStore {
	return &etcdStore{Client: c, ctx: context.Background()}
}

// Check if we can talk to etcd
func (es *etcdStore) Ping() error {
	_, err := etcd.NewMembersAPI(es.EtcdClient()).List(es.ctx)
	return err
}

const ROOT = "/weave-flux/service/"

func serviceRootKey(serviceName string) string {
	return ROOT + serviceName
}

func serviceKey(serviceName string) string {
	return fmt.Sprintf("%s%s/details", ROOT, serviceName)
}

func ruleKey(serviceName, ruleName string) string {
	return fmt.Sprintf("%s%s/groupspec/%s", ROOT, serviceName, ruleName)
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

type parsedRuleKey struct {
	serviceName string
	ruleName    string
}

func (k parsedRuleKey) relevantTo(opts store.QueryServiceOptions) (bool, string) {
	return opts.WithContainerRules, k.serviceName
}

type parsedInstanceKey struct {
	serviceName  string
	instanceName string
}

func (k parsedInstanceKey) relevantTo(opts store.QueryServiceOptions) (bool, string) {
	return opts.WithInstances, k.serviceName
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
			return parsedRuleKey{p[0], p[2]}
		}

	case "instance":
		if len(p) == 3 {
			return parsedInstanceKey{p[0], p[2]}
		}
	}

	return nil
}

func (es *etcdStore) CheckRegisteredService(serviceName string) error {
	_, err := es.Get(es.ctx, serviceRootKey(serviceName), nil)
	return err
}

func (es *etcdStore) AddService(name string, details data.Service) error {
	json, err := json.Marshal(&details)
	if err != nil {
		return fmt.Errorf("Failed to encode: %s", err)
	}

	_, err = es.Set(es.ctx, serviceKey(name), string(json), nil)
	return err
}

func (es *etcdStore) RemoveService(serviceName string) error {
	return es.deleteRecursive(serviceRootKey(serviceName))
}

func (es *etcdStore) RemoveAllServices() error {
	return es.deleteRecursive(ROOT)
}

func (es *etcdStore) deleteRecursive(key string) error {
	_, err := es.Delete(es.ctx, key,
		&etcd.DeleteOptions{Recursive: true})
	return err
}

func (es *etcdStore) GetService(serviceName string, opts store.QueryServiceOptions) (*store.ServiceInfo, error) {
	node, _, err := es.getDirNode(serviceRootKey(serviceName), false,
		opts.WithInstances || opts.WithContainerRules)
	if err != nil {
		return nil, err
	}

	return serviceInfoFromNode(serviceName, node, opts)
}

func (es *etcdStore) GetAllServices(opts store.QueryServiceOptions) ([]*store.ServiceInfo, error) {
	node, _, err := es.getDirNode(ROOT, true, true)
	if err != nil {
		return nil, err
	}

	var svcs []*store.ServiceInfo

	for name, n := range indexDir(node) {
		if !n.Dir {
			continue
		}

		svc, err := serviceInfoFromNode(name, n, opts)
		if err != nil {
			return nil, err
		}

		svcs = append(svcs, svc)
	}

	return svcs, nil
}

func (es *etcdStore) getDirNode(key string, missingOk bool, recursive bool) (*etcd.Node, uint64, error) {
	resp, err := es.Get(es.ctx, key,
		&etcd.GetOptions{Recursive: recursive})
	if err != nil {
		if cerr, ok := err.(etcd.Error); ok && cerr.Code == etcd.ErrorCodeKeyNotFound && missingOk {
			return nil, cerr.Index, nil
		}

		return nil, 0, err
	}

	if !resp.Node.Dir {
		return nil, 0, fmt.Errorf("expected a dir at etcd key %s", key)
	}

	return resp.Node, resp.Index, nil
}

func indexDir(node *etcd.Node) map[string]*etcd.Node {
	res := make(map[string]*etcd.Node)

	if node != nil {
		for _, n := range node.Nodes {
			key := n.Key
			lastSlash := strings.LastIndex(key, "/")
			if lastSlash >= 0 {
				key = key[lastSlash+1:]
			}

			res[key] = n
		}
	}

	return res
}

func serviceInfoFromNode(name string, node *etcd.Node, opts store.QueryServiceOptions) (*store.ServiceInfo, error) {
	dir := indexDir(node)

	details := dir["details"]
	if details == nil {
		return nil, fmt.Errorf("missing services details in etcd node %s", node.Key)
	}

	var err error
	svc := &store.ServiceInfo{
		Name:    name,
		Service: unmarshalService(details, &err),
	}

	if opts.WithInstances {
		for name, n := range indexDir(dir["instance"]) {
			svc.Instances = append(svc.Instances,
				store.InstanceInfo{
					Name:     name,
					Instance: unmarshalInstance(n, &err),
				})
		}
	}

	if opts.WithContainerRules {
		for name, n := range indexDir(dir["groupspec"]) {
			svc.ContainerRules = append(svc.ContainerRules,
				store.ContainerRuleInfo{
					Name:          name,
					ContainerRule: unmarshalRule(n, &err),
				})
		}
	}

	if err != nil {
		return nil, err
	}

	return svc, nil
}

func unmarshalService(node *etcd.Node, errp *error) data.Service {
	var svc data.Service

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &svc)
	}

	return svc
}

func unmarshalRule(node *etcd.Node, errp *error) data.ContainerRule {
	var gs data.ContainerRule

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &gs)
	}

	return gs
}

func unmarshalInstance(node *etcd.Node, errp *error) data.Instance {
	var instance data.Instance

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &instance)
	}

	return instance
}

func (es *etcdStore) SetContainerRule(serviceName string, ruleName string, spec data.ContainerRule) error {
	return es.setJSON(ruleKey(serviceName, ruleName), spec)
}

func (es *etcdStore) RemoveContainerRule(serviceName string, ruleName string) error {
	return es.deleteRecursive(ruleKey(serviceName, ruleName))
}

func (es *etcdStore) AddInstance(serviceName string, instanceName string, inst data.Instance) error {
	return es.setJSON(instanceKey(serviceName, instanceName), inst)
}

func (es *etcdStore) RemoveInstance(serviceName, instanceName string) error {
	return es.deleteRecursive(instanceKey(serviceName, instanceName))
}

func (es *etcdStore) setJSON(key string, val interface{}) error {
	json, err := json.Marshal(val)
	if err != nil {
		return err
	}

	_, err = es.Set(es.ctx, key, string(json), nil)
	return err
}

func (es *etcdStore) WatchServices(ctx context.Context, resCh chan<- data.ServiceChange, errorSink daemon.ErrorSink, opts store.QueryServiceOptions) {
	if ctx == nil {
		ctx = es.ctx
	}

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

			case interface {
				relevantTo(opts store.QueryServiceOptions) (bool, string)
			}:
				if relevant, service := key.relevantTo(opts); relevant {
					resCh <- data.ServiceChange{service, false}
				}
			}

		case "set":
			switch key := parseKey(r.Node.Key).(type) {
			case parsedServiceKey:
				svcs[key.serviceName] = struct{}{}
				resCh <- data.ServiceChange{key.serviceName, false}

			case interface {
				relevantTo(opts store.QueryServiceOptions) (bool, string)
			}:
				if relevant, service := key.relevantTo(opts); relevant {
					resCh <- data.ServiceChange{service, false}
				}
			}
		}
	}

	// Get the initial service list, so that we can report them as
	// deleted if the root node is deleted.  This also gets the
	// initial index for the watch. (Though perhaps that should
	// really be based on the ModifieedIndex of the nodes
	// themselves?)
	node, startIndex, err := es.getDirNode(ROOT, true, false)
	if err != nil {
		errorSink.Post(err)
		return
	}

	for name := range indexDir(node) {
		svcs[name] = struct{}{}
	}
	go func() {
		watcher := es.Watcher(ROOT,
			&etcd.WatcherOptions{
				AfterIndex: startIndex,
				Recursive:  true,
			})

		for {
			next, err := watcher.Next(ctx)
			if err != nil {
				if err != context.Canceled {
					errorSink.Post(err)
				}
				break
			}

			handleResponse(next)
		}
	}()
}
