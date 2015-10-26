package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/bboreham/coatl/backends"
	"github.com/bboreham/coatl/data"

	"github.com/gorilla/mux"
)

func main() {
	etcd := os.Getenv("ETCD_ADDRESS")
	if etcd == "" {
		etcd = "http://localhost:4001"
	}

	back := backends.NewBackend([]string{etcd})
	if err := back.Ping(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Connected to backend\n")
	api := &api{back}

	router := mux.NewRouter()

	router.HandleFunc("/", homePage)
	router.HandleFunc("/index.html", homePage)
	router.PathPrefix("/res/").HandlerFunc(handleResource)

	router.HandleFunc("/api/{service}/", api.listInstances)
	router.HandleFunc("/api/", api.listServices)

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
	backend *backends.Backend
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

	api.backend.ForeachServiceInstance(func(name string, details data.Service) {
		currentService = serviceDetails{
			Name:    name,
			Details: details,
		}
		services = append(services, currentService)
	}, func(_ string, _ data.Instance) {
	})
	json.NewEncoder(w).Encode(services)
}

func (api *api) listInstances(w http.ResponseWriter, r *http.Request) {
	args := mux.Vars(r)
	serviceName := args["service"]
	details, err := api.backend.GetServiceDetails(serviceName)
	if err != nil {
		http.NotFound(w, r)
	}
	children := []instanceDetails{}
	api.backend.ForeachInstance(serviceName, func(name string, details data.Instance) {
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
