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

var testService = data.Service{
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

func TestServices(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	require.Nil(t, be.AddService("svc", testService))
	svc2, err := be.GetServiceDetails("svc")
	require.Nil(t, err)
	require.Equal(t, testService, svc2)

	require.Nil(t, be.CheckRegisteredService("svc"))

	services := func() map[string]data.Service {
		svcs := make(map[string]data.Service)
		require.Nil(t, be.ForeachServiceInstance(func(n string, s data.Service) {
			svcs[n] = s
		}, nil))
		return svcs
	}

	require.Equal(t, map[string]data.Service{"svc": testService}, services())

	require.Nil(t, be.RemoveService("svc"))
	require.Equal(t, map[string]data.Service{}, services())

	require.Nil(t, be.AddService("svc", testService))
	require.Nil(t, be.RemoveAllServices())
	require.Equal(t, map[string]data.Service{}, services())
}

var testInst = data.Instance{
	InstanceGroup: "group",
	Address:       "1.2.3.4",
	Port:          12345,
	Labels:        map[string]string{"key": "val"},
}

func TestInstances(t *testing.T) {
	etcd, be := startEtcd(t)
	defer func() { require.Nil(t, etcd.Destroy()) }()

	require.Nil(t, be.AddService("svc", testService))
	require.Nil(t, be.AddInstance("svc", "inst", testInst))

	instances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, be.ForeachInstance("svc", func(n string, inst data.Instance) {
			insts[n] = inst
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"inst": testInst}, instances())

	serviceInstances := func() map[string]data.Instance {
		insts := make(map[string]data.Instance)
		require.Nil(t, be.ForeachServiceInstance(nil, func(sn string, in string, inst data.Instance) {
			insts[sn+" "+in] = inst
		}))
		return insts
	}

	require.Equal(t, map[string]data.Instance{"svc inst": testInst}, serviceInstances())

	require.Nil(t, be.RemoveInstance("svc", "inst"))
	require.Equal(t, map[string]data.Instance{}, instances())
	require.Equal(t, map[string]data.Instance{}, serviceInstances())
}
