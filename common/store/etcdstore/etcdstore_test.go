package etcdstore

import (
	"fmt"
	"testing"

	etcd "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/etcdutil"
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

	c, err := etcd.New(etcd.Config{Endpoints: []string{
		fmt.Sprintf("http://localhost:%d", server.Port)}})
	require.Nil(t, err)
	test.RunStoreTestSuite(newEtcdStore(etcdutil.NewClient(c)), t)
}
