package etcdstore

import (
	"encoding/json"

	etcd "github.com/coreos/etcd/client"
)

type SessionValue struct {
	Session string
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

	if err = es.cullSessionValues(hostRoot, live); err != nil {
		return err
	}

	serviceRoot, _, err := es.getDirNode(SERVICE_ROOT, true, true)
	if err != nil {
		return err
	}

	for _, svcNode := range serviceRoot.Nodes {
		svcDir := indexDir(svcNode)

		if svcDir[INSTANCE_PATH] != nil {
			if err = es.cullSessionValues(svcDir[INSTANCE_PATH], live); err != nil {
				return err
			}
		}

		if svcDir[INGRESS_INSTANCE_PATH] != nil {
			if err = es.cullSessionValues(svcDir[INGRESS_INSTANCE_PATH], live); err != nil {
				return err
			}
		}
	}

	return nil
}

func (es *etcdStore) cullSessionValues(dir *etcd.Node, live map[string]struct{}) error {
	for _, node := range dir.Nodes {
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
