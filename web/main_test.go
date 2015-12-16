package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/inmem"
)

func doRequest(t *testing.T, st store.Store, url string) *httptest.ResponseRecorder {
	api := api{store: st}
	req, err := http.NewRequest("GET", url, nil)
	require.Nil(t, err)
	resp := httptest.NewRecorder()
	api.router().ServeHTTP(resp, req)
	return resp
}

var testService = data.Service{
	Address:  "1.2.3.4",
	Port:     1234,
	Protocol: "tcp",
}

func TestListServices(t *testing.T) {
	st := inmem.NewInMemStore()
	st.AddService("svc", testService)

	resp := doRequest(t, st, "/api/")
	require.Equal(t, 200, resp.Code)

	var deets []serviceDetails
	require.Nil(t, json.Unmarshal(resp.Body.Bytes(), &deets))
	require.Equal(t, []serviceDetails{serviceDetails{Name: "svc", Service: testService}}, deets)
}

var testInstance = data.Instance{
	ContainerGroup: "group",
	Address:        "1.2.3.4",
	Port:           12345,
	Labels:         map[string]string{"key": "val"},
}

func TestListInstances(t *testing.T) {
	st := inmem.NewInMemStore()

	resp := doRequest(t, st, "/api/nosuchservice/")
	require.Equal(t, 404, resp.Code)

	st.AddService("svc", testService)
	st.AddInstance("svc", "inst", testInstance)

	resp = doRequest(t, st, "/api/svc/")
	require.Equal(t, 200, resp.Code)

	var svc service
	require.Nil(t, json.Unmarshal(resp.Body.Bytes(), &svc))
	require.Equal(t, service{
		serviceDetails: serviceDetails{
			Name:    "svc",
			Service: testService,
		},
		Children: []instanceDetails{{
			Name:     "inst",
			Instance: testInstance,
		}},
	}, svc)
}
