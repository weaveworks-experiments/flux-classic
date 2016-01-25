package prometheus

import (
	"net"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/squaremo/flux/balancer/events"
	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/etcdutil"
)

type Config struct {
	ListenAddr    string
	AdvertiseAddr string
	EtcdClient    etcdutil.Client
	ErrorSink     daemon.ErrorSink
}

type handler struct {
	listenAddr string
	errorSink  daemon.ErrorSink

	events.DiscardOthers
	connections   *prom.CounterVec
	http          *prom.CounterVec
	httpRoundtrip *prom.SummaryVec
	httpTotal     *prom.SummaryVec

	advertiser *advertiser
	listener   net.Listener
}

func NewEventHandler(cf Config) (events.Handler, error) {
	h := &handler{listenAddr: cf.ListenAddr, errorSink: cf.ErrorSink}

	h.connections = prom.NewCounterVec(prom.CounterOpts{
		Name: "flux_connections_total",
		Help: "Number of TCP connections established",
	}, []string{"individual", "group", "src", "dst", "protocol"})

	httpLabels := []string{"individual", "group", "src", "dst", "method", "code"}

	h.http = prom.NewCounterVec(prom.CounterOpts{
		Name: "flux_http_total",
		Help: "Number of HTTP request/response exchanges",
	}, httpLabels)

	h.httpRoundtrip = prom.NewSummaryVec(prom.SummaryOpts{
		Name: "flux_http_roundtrip_usec",
		Help: "HTTP response roundtrip time in microseconds",
	}, httpLabels)

	h.httpTotal = prom.NewSummaryVec(prom.SummaryOpts{
		Name: "flux_http_total_usec",
		Help: "HTTP total response time in microseconds",
	}, httpLabels)

	if cf.AdvertiseAddr != "" {
		var err error
		if h.advertiser, err = newAdvertiser(cf); err != nil {
			return nil, err
		}
	}

	return h, nil
}

func (h *handler) collectors() []prom.Collector {
	return []prom.Collector{h.connections, h.http, h.httpRoundtrip,
		h.httpTotal}
}

func (h *handler) Start() {
	for _, c := range h.collectors() {
		if err := prom.Register(c); err != nil {
			h.errorSink.Post(err)
			return
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())

	var err error
	h.listener, err = net.Listen("tcp", h.listenAddr)
	if err != nil {
		h.errorSink.Post(err)
		return
	}

	go func() { log.Error(http.Serve(h.listener, mux)) }()

	if h.advertiser != nil {
		h.advertiser.start()
	}
}

func (h *handler) Stop() {
	if h.advertiser != nil {
		h.advertiser.stop()
	}

	if listener := h.listener; listener != nil {
		h.listener = nil
		if err := listener.Close(); err != nil {
			h.errorSink.Post(err)
		}
	}

	for _, c := range h.collectors() {
		prom.Unregister(c)
	}
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
