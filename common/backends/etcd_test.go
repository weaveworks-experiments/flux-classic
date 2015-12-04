package backends

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/test/embeddedetcd"
)

func startEtcd(t *testing.T) (*embeddedetcd.SimpleEtcd, *Backend) {
	etcd, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	be := NewBackend(fmt.Sprintf("http://localhost:%d", etcd.Port))
	require.Nil(t, be.Ping())
	return etcd, be
}

func TestEtcd(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()
	require.Nil(t, be.Ping())
}

func TestAddService(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	svc := data.Service{
		Address:  "1.2.3.4",
		Port:     1234,
		Protocol: "tcp",
		InstanceSpecs: map[data.InstanceGroup]data.InstanceSpec{
			"group": {
				AddressSpec: data.AddressSpec{
					Type: "foo",
					Port: 5678,
				},
				Selector: data.Selector{
					"foo": "bar",
				},
			},
		},
	}

	require.Nil(t, be.AddService("svc", svc))
	svc2, err := be.GetServiceDetails("svc")
	require.Nil(t, err)
	require.Equal(t, svc, svc2)
}
