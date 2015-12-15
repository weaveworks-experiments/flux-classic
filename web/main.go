package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/etcdstore"

	"github.com/gorilla/mux"
)

func main() {
	prom := os.Getenv("PROM_ADDRESS")
	if prom == "" {
		prom = "http://localhost:9090"
	}

	store := etcdstore.NewFromEnv()
	if err := store.Ping(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Connected to backend\n")
	api := &api{store, prom}

	router := mux.NewRouter()

	router.HandleFunc("/", homePage)
	router.HandleFunc("/index.html", homePage)
	router.PathPrefix("/res/").HandlerFunc(handleResource)

	router.HandleFunc("/api/{service}/", api.listInstances)
	router.HandleFunc("/api/", api.listServices)

	router.PathPrefix("/stats/").HandlerFunc(api.proxyStats)

	http.ListenAndServe("0.0.0.0:7070", router)
}

func handleResource(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[1:]
	http.ServeFile(w, r, file)
}

func homePage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

//=== API handlers

type api struct {
	store   store.Store
	promURL string
}

type serviceDetails struct {
	Name    string       `json:"name"`
	Details data.Service `json:"details"`
}

type service struct {
	Name     string            `json:"name"`
	Children []instanceDetails `json:"children"`
	Details  data.Service      `json:"details"`
}

type instanceDetails struct {
	Name    string        `json:"name"`
	Details data.Instance `json:"details"`
}

func (api *api) listServices(w http.ResponseWriter, r *http.Request) {
	var currentService serviceDetails
	services := []serviceDetails{}

	api.store.ForeachServiceInstance(func(name string, details data.Service) {
		currentService = serviceDetails{
			Name:    name,
			Details: details,
		}
		services = append(services, currentService)
	}, nil)
	json.NewEncoder(w).Encode(services)
}

func (api *api) listInstances(w http.ResponseWriter, r *http.Request) {
	args := mux.Vars(r)
	serviceName := args["service"]
	details, err := api.store.GetServiceDetails(serviceName)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	children := []instanceDetails{}
	api.store.ForeachInstance(serviceName, func(name string, details data.Instance) {
		instance := instanceDetails{
			Name:    name,
			Details: details,
		}
		children = append(children, instance)
	})
	service := service{
		Name:     serviceName,
		Details:  details,
		Children: children,
	}
	json.NewEncoder(w).Encode(service)
}

/* Proxy for prometheus, as a stop-gap */

func (api *api) proxyStats(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/stats"):] + "?" + r.URL.RawQuery
	log.Println(path)
	resp, err := http.Get(api.promURL + path)
	if err != nil {
		http.Error(w, "Error contacting prometheus server: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	for k, vs := range resp.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
