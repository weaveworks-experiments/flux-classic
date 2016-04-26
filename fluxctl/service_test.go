package main

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
)

func allServices(t *testing.T, st store.Store) map[string]*store.ServiceInfo {
	services, err := st.GetAllServices(store.QueryServiceOptions{})
	require.NoError(t, err)
	return services
}

func TestService(t *testing.T) {
	_, err := runOpts(&addOpts{}, []string{})
	require.Error(t, err)
}

func TestMinimal(t *testing.T) {
	st, err := runOpts(&addOpts{}, []string{"foo"})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	require.NotNil(t, services["foo"])
}

func TestParseAddress(t *testing.T) {
	svc, err := parseAddress("10.3.4.5")
	require.Error(t, err)

	svc, err = parseAddress("192.168.45.76:8000")
	require.NoError(t, err)
	require.Equal(t, store.Service{
		Address:  &netutil.IPPort{net.ParseIP("192.168.45.76"), 8000},
		Protocol: "",
	}, svc)
}

func TestServiceAddress(t *testing.T) {
	st, err := runOpts(&addOpts{}, []string{
		"foo", "--address", "10.3.4.5:8000", "--instance-port", "7777"})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	require.NotNil(t, services["foo"])
	require.Equal(t, &netutil.IPPort{net.ParseIP("10.3.4.5"), 8000}, services["foo"].Address)
	require.Equal(t, 7777, services["foo"].InstancePort)
}

func TestServiceSelect(t *testing.T) {
	st, err := runOpts(&addOpts{}, []string{
		"svc", "--image", "repo/image",
	})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	svc, err := st.GetService("svc", store.QueryServiceOptions{WithContainerRules: true})
	require.NoError(t, err)
	specs := svc.ContainerRules
	require.Len(t, specs, 1)
	require.Equal(t, store.ContainerRule{
		Selector: map[string]string{
			"image": "repo/image",
		},
	}, specs[DEFAULT_GROUP])
}
