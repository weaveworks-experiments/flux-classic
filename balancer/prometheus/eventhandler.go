package prometheus

import (
	"net"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/squaremo/flux/balancer/events"
)

type handler struct {
	events.DiscardOthers
	connections   *prom.CounterVec
	http          *prom.CounterVec
	httpRoundtrip *prom.SummaryVec
	httpTotal     *prom.SummaryVec
}

func NewEventHandler(address string) (events.Handler, error) {
	connectionCounter := prom.NewCounterVec(prom.CounterOpts{
		Name: "flux_connections_total",
		Help: "Number of TCP connections established",
	}, []string{"individual", "group", "src", "dst", "protocol"})

	httpLabels := []string{"individual", "group", "src", "dst", "method", "code"}

	httpCounter := prom.NewCounterVec(prom.CounterOpts{
		Name: "flux_http_total",
		Help: "Number of HTTP request/response exchanges",
	}, httpLabels)

	httpRoundtrip := prom.NewSummaryVec(prom.SummaryOpts{
		Name: "flux_http_roundtrip_usec",
		Help: "HTTP response roundtrip time in microseconds",
	}, httpLabels)

	httpTotal := prom.NewSummaryVec(prom.SummaryOpts{
		Name: "flux_http_total_usec",
		Help: "HTTP total response time in microseconds",
	}, httpLabels)

	for _, m := range []prom.Collector{connectionCounter, httpCounter, httpRoundtrip, httpTotal} {
		if err := prom.Register(m); err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	go func() { log.Error(http.Serve(listener, mux)) }()

	return &handler{
		connections:   connectionCounter,
		http:          httpCounter,
		httpRoundtrip: httpRoundtrip,
		httpTotal:     httpTotal,
	}, nil
}

func (h *handler) Connection(ev *events.Connection) {
	h.connections.WithLabelValues(ev.Instance.Name, ev.Instance.Group, ev.Inbound.IP.String(), ev.Instance.IP.String(), ev.Service.Protocol).Inc()
}

func (h *handler) HttpExchange(ev *events.HttpExchange) {
	instName := ev.Instance.Name
	group := ev.Instance.Group
	src := ev.Inbound.IP.String()
	dst := ev.Instance.IP.String()
	method := ev.Request.Method
	code := strconv.Itoa(ev.Response.StatusCode)
	h.http.WithLabelValues(instName, group, src, dst, method, code).Inc()
	h.httpRoundtrip.WithLabelValues(instName, group, src, dst, method, code).Observe(float64(ev.RoundTrip / time.Microsecond))
	h.httpTotal.WithLabelValues(instName, group, src, dst, method, code).Observe(float64(ev.TotalTime / time.Microsecond))
}
