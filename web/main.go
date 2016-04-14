package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
	"github.com/weaveworks/flux/common/version"

	"github.com/gorilla/mux"
)

func main() {
	log.Println(version.Banner())
	prom := os.Getenv("PROMETHEUS_ADDRESS")
	if prom == "" {
		log.Fatal("PROMETHEUS_ADDRESS environment variable not set")
	}

	store, err := etcdstore.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Connected to backend\n")
	api := &api{store, prom}

	http.ListenAndServe("0.0.0.0:7070", api.router())
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

func (api *api) router() http.Handler {
	router := mux.NewRouter()

	router.HandleFunc("/", homePage)
	router.HandleFunc("/index.html", homePage)
	router.PathPrefix("/assets/").HandlerFunc(handleResource)

	router.HandleFunc("/api/services", api.allServices)
	router.PathPrefix("/stats/").HandlerFunc(api.proxyStats)

	return router
}

// List all services, along with their instances and accompanying
// metastore.

func (api *api) allServices(w http.ResponseWriter, r *http.Request) {
	services, err := api.store.GetAllServices(store.QueryServiceOptions{WithInstances: true})
	if err != nil {
		http.Error(w, "Error getting services from store: "+err.Error(), 500)
	}
	json.NewEncoder(w).Encode(&services)
}

/* Proxy for prometheus, as a stop-gap */

func (api *api) proxyStats(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/stats"):] + "?" + r.URL.RawQuery
	resp, err := http.Get(api.promURL + path)
	if err != nil {
		log.Printf("Error forwarding to prometheus at %s: %s", path, err)
		return
	}

	if resp.StatusCode != 200 {
		log.Printf("Request to prometheus at %s: %d response", path, resp.StatusCode)
	}

	defer resp.Body.Close()
	for k, vs := range resp.Header {
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
