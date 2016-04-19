package etcdstore

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/store"
)

type etcdStore struct {
	etcdutil.Client
	ctx     context.Context
	session string
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
	session := makeSessionID()
	return &etcdStore{Client: c, ctx: context.Background(), session: session}
}

func makeSessionID() string {
	bytes := make([]byte, 160/8)
	rand.Read(bytes)
	return base32.HexEncoding.EncodeToString(bytes)
}

// Check if we can talk to etcd
func (es *etcdStore) Ping() error {
	_, err := etcd.NewMembersAPI(es.EtcdClient()).List(es.ctx)
	return err
}

const ROOT = "/weave-flux/"
const SERVICE_ROOT = ROOT + "service/"
const HOST_ROOT = ROOT + "host/"
const SESSION_ROOT = ROOT + "session/"

func serviceRootKey(serviceName string) string {
	return SERVICE_ROOT + serviceName
}

func serviceKey(serviceName string) string {
	return fmt.Sprintf("%s%s/details", SERVICE_ROOT, serviceName)
}

func ruleKey(serviceName, ruleName string) string {
	return fmt.Sprintf("%s%s/groupspec/%s", SERVICE_ROOT, serviceName, ruleName)
}

func instanceKey(serviceName, instanceName string) string {
	return fmt.Sprintf("%s%s/instance/%s", SERVICE_ROOT, serviceName, instanceName)
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
	if len(key) <= len(SERVICE_ROOT) {
		return parsedRootKey{}
	}

	p := strings.Split(key[len(SERVICE_ROOT):], "/")
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

func (es *etcdStore) AddService(name string, details store.Service) error {
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
	return es.deleteRecursive(SERVICE_ROOT)
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
	node, _, err := es.getDirNode(SERVICE_ROOT, true, true)
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

func unmarshalService(node *etcd.Node, errp *error) store.Service {
	var svc store.Service

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &svc)
	}

	return svc
}

func unmarshalRule(node *etcd.Node, errp *error) store.ContainerRule {
	var gs store.ContainerRule

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &gs)
	}

	return gs
}

func unmarshalInstance(node *etcd.Node, errp *error) store.Instance {
	var instance store.Instance

	if *errp == nil {
		*errp = json.Unmarshal([]byte(node.Value), &instance)
	}

	return instance
}

func (es *etcdStore) SetContainerRule(serviceName string, ruleName string, spec store.ContainerRule) error {
	return es.setJSON(ruleKey(serviceName, ruleName), spec)
}

func (es *etcdStore) RemoveContainerRule(serviceName string, ruleName string) error {
	return es.deleteRecursive(ruleKey(serviceName, ruleName))
}

func (es *etcdStore) AddInstance(serviceName string, instanceName string, inst store.Instance) error {
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

func (es *etcdStore) WatchServices(ctx context.Context, resCh chan<- store.ServiceChange, errorSink daemon.ErrorSink, opts store.QueryServiceOptions) {
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
					resCh <- store.ServiceChange{
						Name:           name,
						ServiceDeleted: true,
					}
				}
				svcs = make(map[string]struct{})

			case parsedServiceRootKey:
				delete(svcs, key.serviceName)
				resCh <- store.ServiceChange{
					Name:           key.serviceName,
					ServiceDeleted: true,
				}

			case interface {
				relevantTo(opts store.QueryServiceOptions) (bool, string)
			}:
				if relevant, service := key.relevantTo(opts); relevant {
					resCh <- store.ServiceChange{
						Name:           service,
						ServiceDeleted: false,
					}
				}
			}

		case "set":
			switch key := parseKey(r.Node.Key).(type) {
			case parsedServiceKey:
				svcs[key.serviceName] = struct{}{}
				resCh <- store.ServiceChange{
					Name:           key.serviceName,
					ServiceDeleted: false,
				}

			case interface {
				relevantTo(opts store.QueryServiceOptions) (bool, string)
			}:
				if relevant, service := key.relevantTo(opts); relevant {
					resCh <- store.ServiceChange{
						Name:           service,
						ServiceDeleted: false,
					}
				}
			}
		}
	}

	// Get the initial service list, so that we can report them as
	// deleted if the root node is deleted.  This also gets the
	// initial index for the watch. (Though perhaps that should
	// really be based on the ModifieedIndex of the nodes
	// themselves?)
	node, startIndex, err := es.getDirNode(SERVICE_ROOT, true, false)
	if err != nil {
		errorSink.Post(err)
		return
	}

	for name := range indexDir(node) {
		svcs[name] = struct{}{}
	}
	go func() {
		watcher := es.Watcher(SERVICE_ROOT,
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

/* Host methods */

func hostKey(identity string) string {
	return HOST_ROOT + identity
}

type sessionHost struct {
	*store.Host
	Session string
}

func (es *etcdStore) RegisterHost(identity string, details *store.Host) error {
	sh := sessionHost{
		Host:    details,
		Session: es.session,
	}
	return es.setJSON(hostKey(identity), sh)
}

func (es *etcdStore) DeregisterHost(identity string) error {
	return es.deleteRecursive(hostKey(identity))
}

func (es *etcdStore) GetHosts() ([]*store.Host, error) {
	live, err := es.liveSessions()
	if err != nil {
		return nil, err
	}

	node, _, err := es.getDirNode(HOST_ROOT, true, false)
	if err != nil {
		return nil, err
	}

	var hosts []*store.Host

	for _, n := range indexDir(node) {
		host, err := hostFromNode(n)
		if err != nil {
			return nil, err
		}
		if _, found := live[host.Session]; found {
			hosts = append(hosts, host.Host)
		}
	}

	return hosts, nil
}

func hostFromNode(node *etcd.Node) (*sessionHost, error) {
	var host sessionHost
	return &host, json.Unmarshal([]byte(node.Value), &host)
}

func (es *etcdStore) WatchHosts(ctx context.Context, changes chan<- store.HostChange, errs daemon.ErrorSink) {
	if ctx == nil {
		ctx = es.ctx
	}

	hosts := make(map[string]struct{})
	node, startIndex, err := es.getDirNode(HOST_ROOT, true, false)
	if err != nil {
		errs.Post(err)
		return
	}

	handleResponse := func(r *etcd.Response) {
		fmt.Printf("Change %+v\n", r)
		hostID := r.Node.Key[len(HOST_ROOT):]
		switch r.Action {
		case "delete", "expire":
			delete(hosts, hostID)
			changes <- store.HostChange{
				Name:         hostID,
				HostDeparted: true,
			}

		case "set":
			hosts[hostID] = struct{}{}
			changes <- store.HostChange{
				Name:         hostID,
				HostDeparted: false,
			}
		}
	}

	for name := range indexDir(node) {
		hosts[name] = struct{}{}
	}
	go func() {
		watcher := es.Watcher(HOST_ROOT,
			&etcd.WatcherOptions{
				AfterIndex: startIndex,
				Recursive:  true,
			})
		for {
			next, err := watcher.Next(ctx)
			if err != nil {
				if err != context.Canceled {
					errs.Post(err)
				}
				break
			}
			handleResponse(next)
		}
	}()
}

/* store.Cluster methods */

func sessionKey(id string) string {
	return SESSION_ROOT + id
}

func (es *etcdStore) Heartbeat(ttl time.Duration) error {
	_, err := es.Set(es.ctx, sessionKey(es.session), es.session, &etcd.SetOptions{TTL: ttl})
	return err
}

func (es *etcdStore) EndSession() error {
	_, err := es.Delete(es.ctx, sessionKey(es.session), &etcd.DeleteOptions{Recursive: false})
	return err
}

func (es *etcdStore) liveSessions() (map[string]struct{}, error) {
	node, _, err := es.getDirNode(SESSION_ROOT, true, false)
	if err != nil {
		return nil, err
	}
	live := map[string]struct{}{}
	for name := range indexDir(node) {
		live[name] = struct{}{}
	}
	return live, nil
}
