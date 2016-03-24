package prometheus

import (
	"net"
	"net/http"
	"strconv"
	"time"

	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"
	prom "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/netutil"
)

type Config struct {
	ListenAddr    string
	AdvertiseAddr string
	EtcdClient    etcdutil.Client
}

type EventHandler struct {
	errs       daemon.ErrorSink
	finished   chan struct{}
	advertiser daemon.Component

	events.DiscardOthers
	connections   *prom.CounterVec
	http          *prom.CounterVec
	httpRoundtrip *prom.SummaryVec
	httpTotal     *prom.SummaryVec

	listener net.Listener
}

func (cf Config) Prepare() (func(daemon.ErrorSink) events.Handler, error) {
	startAdvertiser, err := cf.advertiseStartFunc()
	if err != nil {
		return nil, err
	}

	return func(errs daemon.ErrorSink) events.Handler {
		h := &EventHandler{
			errs:     errs,
			finished: make(chan struct{}),
		}
		errs.Post(h.start(cf.ListenAddr))
		h.advertiser = startAdvertiser(errs)
		return h
	}, nil
}

func (h *EventHandler) start(listenAddr string) error {
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

	for _, c := range h.collectors() {
		if err := prom.Register(c); err != nil {
			return err
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())

	var err error
	h.listener, err = net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	go func() {
		defer close(h.finished)
		log.Error(http.Serve(h.listener, mux))
	}()

	return nil
}

func (h *EventHandler) Stop() {
	h.advertiser.Stop()

	if h.listener != nil {
		h.errs.Post(h.listener.Close())
	}

	for _, c := range h.collectors() {
		prom.Unregister(c)
	}

	<-h.finished
}

func (h *EventHandler) collectors() []prom.Collector {
	return []prom.Collector{h.connections, h.http, h.httpRoundtrip,
		h.httpTotal}
}

func (h *EventHandler) Connection(ev *events.Connection) {
	h.connections.WithLabelValues(ev.Instance.Name, ev.Instance.Group, ev.Inbound.IP.String(), ev.Instance.IP.String(), ev.Service.Protocol).Inc()
}

func (h *EventHandler) HttpExchange(ev *events.HttpExchange) {
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

const TTL = 5 * time.Minute

func (cf Config) advertiseStartFunc() (daemon.StartFunc, error) {
	if cf.AdvertiseAddr == "" {
		return daemon.NullStartFunc, nil
	}

	address, err := netutil.NormalizeHostPort(cf.AdvertiseAddr,
		"tcp", false)
	if err != nil {
		return nil, err
	}

	run := func(stop <-chan struct{}, errs daemon.ErrorSink) {
		ctx := context.Background()

		resp, err := cf.EtcdClient.CreateInOrder(ctx,
			"/weave-flux/prometheus-targets", address,
			&etcd.CreateInOrderOptions{TTL: TTL})
		if err != nil {
			errs.Post(err)
			return
		}

		key := resp.Node.Key
		t := time.NewTicker(TTL / 2)
		defer t.Stop()

		for {
			select {
			case <-t.C:
			case <-stop:
				return
			}

			_, err := cf.EtcdClient.Set(ctx, key, address,
				&etcd.SetOptions{TTL: TTL})
			if err != nil {
				errs.Post(err)
				return
			}
		}
	}

	return daemon.Restart(TTL/10, daemon.SimpleComponent(run)), nil
}
