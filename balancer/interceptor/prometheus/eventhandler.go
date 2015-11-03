package prometheus

import (
	"net"
	"net/http"

	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/squaremo/ambergreen/balancer/interceptor/events"
)

type handler struct {
	connections prom.Counter
}

func NewEventHandler(address string) (events.Handler, error) {

	connectionCounter := prom.NewCounter(prom.CounterOpts{
		Name: "ambergreen_connections_total",
		Help: "Number of TCP connections established through balancer",
	})
	if err := prom.Register(connectionCounter); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	// Don't care about an error from this
	go http.Serve(listener, mux)

	return &handler{
		connections: connectionCounter,
	}, nil
}

func (h *handler) Connection(ev *events.Connection) {
	h.connections.Inc()
}

func (h *handler) HttpExchange(ev *events.HttpExchange) {
}
