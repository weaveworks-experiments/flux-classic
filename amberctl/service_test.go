package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/inmem"
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

type service struct {
	data.Service
	Name string
}

func allServices(st store.Store) []service {
	services := make([]service, 0)
	st.ForeachServiceInstance(func(name string, svc data.Service) {
		services = append(services, service{Service: svc, Name: name})
	}, nil)
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
	services := allServices(st)
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

func TestAddress(t *testing.T) {
	st, err := runCmd([]string{
		"foo", "--address", "10.3.4.5:8000"})
	require.NoError(t, err)
	services := allServices(st)
	require.Len(t, services, 1)
	require.Equal(t, "foo", services[0].Name)
	require.Equal(t, "10.3.4.5", services[0].Address)
	require.Equal(t, 8000, services[0].Port)
}
