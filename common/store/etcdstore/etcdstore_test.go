package etcdstore

import (
	"net"
	"testing"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/test"
	"github.com/weaveworks/flux/common/test/embeddedetcd"
)

func (es *etcdStore) Reset(t *testing.T) {
	err := es.deleteRecursive(ROOT)
	if cerr, ok := err.(etcd.Error); ok && cerr.Code == etcd.ErrorCodeKeyNotFound {
		err = nil
	}

	require.Nil(t, err)
}

func TestEtcdStore(t *testing.T) {
	server, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	defer func() { require.Nil(t, server.Destroy()) }()

	c, err := etcdutil.NewClient(server.URL())
	require.Nil(t, err)
	test.RunStoreTestSuite(newEtcdStore(c), t)
}

func TestSessionValues(t *testing.T) {
	server, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	defer func() { require.Nil(t, server.Destroy()) }()

	c, err := etcdutil.NewClient(server.URL())
	require.Nil(t, err)

	es := newEtcdStore(c)

	require.Nil(t, es.Heartbeat(10*time.Second))
	require.Nil(t, es.RegisterHost("test host", &store.Host{IP: net.ParseIP("10.11.23.45")}))
	hostRoot, _, err := es.getDirNode(HOST_ROOT, true, true)
	require.Nil(t, err)
	require.Len(t, indexDir(hostRoot), 1)

	require.Nil(t, es.AddService("test service", store.Service{}))
	require.Nil(t, es.AddInstance("test service", "test instance", store.Instance{}))
	service, _, err := es.getDirNode(serviceRootKey("test service"), true, true)
	require.Nil(t, err)
	require.Len(t, instanceDir(service), 1)

	require.Nil(t, es.EndSession())
	require.Nil(t, es.doCollection())

	hostRoot, _, err = es.getDirNode(HOST_ROOT, true, true)
	require.Nil(t, err)
	require.Len(t, indexDir(hostRoot), 0)

	service, _, err = es.getDirNode(serviceRootKey("test service"), true, true)
	require.Nil(t, err)
	require.Len(t, instanceDir(service), 0)
}
