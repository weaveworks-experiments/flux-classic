package etcdstore

import (
	"fmt"
	"testing"

	etcd_errors "github.com/coreos/etcd/error"
	"github.com/coreos/go-etcd/etcd"
	"github.com/stretchr/testify/require"

	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/test"
	"github.com/squaremo/flux/common/test/embeddedetcd"
)

type testableEtcdStore struct {
	store.Store
	addr string
}

func (tes testableEtcdStore) Reset(t *testing.T) {
	client := etcd.NewClient([]string{tes.addr})
	_, err := client.Delete(ROOT, true)
	if err != nil {
		etcderr, ok := err.(*etcd.EtcdError)
		require.True(t, ok && etcderr.ErrorCode == etcd_errors.EcodeKeyNotFound)
	}
}

func TestEtcdStore(t *testing.T) {
	etcd, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	defer func() { require.Nil(t, etcd.Destroy()) }()
	addr := fmt.Sprintf("http://localhost:%d", etcd.Port)
	test.RunStoreTestSuite(&testableEtcdStore{New(addr), addr}, t)
}
