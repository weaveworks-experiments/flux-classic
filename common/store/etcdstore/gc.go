package etcdstore

import (
	"encoding/json"

	etcd "github.com/coreos/etcd/client"
)

type SessionValue struct {
	Session string
}

func instanceDir(serviceNode *etcd.Node) map[string]*etcd.Node {
	dir := indexDir(serviceNode)
	return indexDir(dir[INSTANCE_PATH])
}

func (es *etcdStore) doCollection() error {
	live, err := es.liveSessions()
	if err != nil {
		return err
	}
	hostRoot, _, err := es.getDirNode(HOST_ROOT, true, true)
	if err != nil {
		return err
	}
	if err = es.cullSessionValues(indexDir(hostRoot), live); err != nil {
		return err
	}

	serviceRoot, _, err := es.getDirNode(SERVICE_ROOT, true, true)
	if err != nil {
		return err
	}
	for _, svc := range indexDir(serviceRoot) {
		if err = es.cullSessionValues(instanceDir(svc), live); err != nil {
			return err
		}
	}

	return nil
}

func (es *etcdStore) cullSessionValues(nodes map[string]*etcd.Node, live map[string]struct{}) error {
	for _, node := range nodes {
		var val SessionValue
		if err := json.Unmarshal([]byte(node.Value), &val); err != nil {
			return err
		}
		if val.Session == "" {
			continue
		}
		if _, isLive := live[val.Session]; !isLive {
			es.Delete(es.ctx, node.Key, &etcd.DeleteOptions{PrevIndex: node.ModifiedIndex})
		}
	}
	return nil
}
