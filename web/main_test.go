package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

func doRequest(t *testing.T, st store.Store, url string) *httptest.ResponseRecorder {
	api := api{store: st}
	req, err := http.NewRequest("GET", url, nil)
	require.Nil(t, err)
	resp := httptest.NewRecorder()
	api.router().ServeHTTP(resp, req)
	return resp
}

var testService = store.Service{
	Address:  &netutil.IPPort{net.ParseIP("1.2.3.4"), 4321},
	Protocol: "tcp",
}

var testInstance = store.Instance{
	ContainerRule: "group",
	Address:       &netutil.IPPort{net.ParseIP("1.2.3.4"), 12345},
	Labels:        map[string]string{"key": "val"},
}

func allServices(t *testing.T, st store.Store) []*store.ServiceInfo {
	services, err := st.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	require.NoError(t, err)
	return services
}

func TestListServices(t *testing.T) {
	st := inmem.NewInMemStore()
	st.AddService("svc", testService)
	st.AddInstance("svc", "inst", testInstance)

	resp := doRequest(t, st, "/api/services")
	require.Equal(t, 200, resp.Code)

	var deets []*store.ServiceInfo
	require.Nil(t, json.Unmarshal(resp.Body.Bytes(), &deets))
	services := allServices(t, st)
	require.Equal(t, services, deets)
}
