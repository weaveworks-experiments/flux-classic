package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/flux/common/data"
	"github.com/squaremo/flux/common/store"
	"github.com/squaremo/flux/common/store/inmem"
)

func runCmd(args []string) (store.Store, error) {
	st := inmem.NewInMemStore()
	add := &addOpts{
		store: st,
	}
	cmd := add.makeCommand()
	cmd.SetArgs(args)
	err := cmd.Execute()
	return st, err
}

func allServices(t *testing.T, st store.Store) []store.ServiceInfo {
	services, err := st.GetAllServices(store.QueryServiceOptions{})
	require.NoError(t, err)
	return services
}

func TestService(t *testing.T) {
	_, err := runCmd([]string{})
	require.Error(t, err)
}

func TestMinimal(t *testing.T) {
	st, err := runCmd([]string{
		"foo"})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	require.Equal(t, "foo", services[0].Name)
}

func TestParseAddress(t *testing.T) {
	svc, err := parseAddress("10.3.4.5")
	require.Error(t, err)

	svc, err = parseAddress("192.168.45.76:8000")
	require.NoError(t, err)
	require.Equal(t, data.Service{
		Address:  "192.168.45.76",
		Port:     8000,
		Protocol: "",
	}, svc)

	svc, err = parseAddress("192.168.45.76:8000/http")
	require.NoError(t, err)
	require.Equal(t, data.Service{
		Address:  "192.168.45.76",
		Port:     8000,
		Protocol: "http",
	}, svc)

}

func TestServiceAddress(t *testing.T) {
	st, err := runCmd([]string{
		"foo", "--address", "10.3.4.5:8000"})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	require.Equal(t, "foo", services[0].Name)
	require.Equal(t, "10.3.4.5", services[0].Address)
	require.Equal(t, 8000, services[0].Port)
}

func TestServiceSelectMissingPortSpec(t *testing.T) {
	_, err := runCmd([]string{
		"svc", "--image", "repo/image",
	})
	require.Error(t, err)
}

func TestServiceSelect(t *testing.T) {
	st, err := runCmd([]string{
		"svc", "--image", "repo/image", "--port-fixed", "9000",
	})
	require.NoError(t, err)
	services := allServices(t, st)
	require.Len(t, services, 1)
	svc, err := st.GetService("svc", store.QueryServiceOptions{WithGroupSpecs: true})
	require.NoError(t, err)
	specs := svc.ContainerGroupSpecs
	require.Len(t, specs, 1)
	spec := specs[0]
	require.NotNil(t, spec)
	require.Equal(t, DEFAULT_GROUP, spec.Name)
	require.Equal(t, data.Selector(map[string]string{
		"image": "repo/image",
	}), spec.Selector)
	require.Equal(t, data.AddressSpec{"fixed", 9000}, spec.AddressSpec)
}
