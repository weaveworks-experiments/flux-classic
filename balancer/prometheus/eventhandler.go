package prometheus

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	etcd "github.com/coreos/etcd/client"
	prom "github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/netutil"
)

type eventHandlerSlot struct {
	slot *events.Handler
}

type eventHandlerKey struct{}

func EventHandlerDependency(slot *events.Handler) daemon.DependencySlot {
	return eventHandlerSlot{slot}
}

func (eventHandlerSlot) Key() daemon.DependencyKey {
	return eventHandlerKey{}
}

func (s eventHandlerSlot) Assign(value interface{}) {
	*s.slot = value.(events.Handler)
}

type eventHandlerConfig struct {
	listenAddr    string
	advertiseAddr string
	etcdClient    etcdutil.Client
	hostIP        net.IP
}

func (eventHandlerKey) MakeConfig() daemon.DependencyConfig {
	return &eventHandlerConfig{}
}

func (cf *eventHandlerConfig) Populate(deps *daemon.Dependencies) {
	deps.StringVar(&cf.listenAddr,
		"listen-prometheus", ":9000",
		"listen for connections from Prometheus on this IP address and port; e.g., :9000")
	deps.StringVar(&cf.advertiseAddr,
		"advertise-prometheus", "",
		"IP address and port to advertise to Prometheus; e.g. 192.168.42.221:9000")

	deps.Dependency(etcdutil.ClientDependency(&cf.etcdClient))
	deps.Dependency(netutil.HostIPDependency(&cf.hostIP))
}

type eventHandler struct {
	*eventHandlerConfig

	events.DiscardOthers
	connections   *prom.CounterVec
	http          *prom.CounterVec
	httpRoundtrip *prom.SummaryVec
	httpTotal     *prom.SummaryVec
}

func (cf *eventHandlerConfig) MakeValue() (interface{}, daemon.StartFunc, error) {
	// Default the prom AdvertiseAddr based on the host IP
	if cf.advertiseAddr == "" {
		_, port, err := net.SplitHostPort(cf.listenAddr)
		if err != nil {
			return nil, nil, err
		}

		cf.advertiseAddr = fmt.Sprintf("%s:%s", cf.hostIP, port)
	} else {
		address, err := netutil.NormalizeHostPort(cf.advertiseAddr,
			"tcp", false)
		if err != nil {
			return nil, nil, err
		}

		cf.advertiseAddr = address
	}

	httpLabels := []string{"individual", "src", "dst", "method", "code"}
	h := &eventHandler{
		eventHandlerConfig: cf,

		connections: prom.NewCounterVec(prom.CounterOpts{
			Name: "flux_connections_total",
			Help: "Number of TCP connections established",
		}, []string{"individual", "src", "dst", "protocol"}),

		http: prom.NewCounterVec(prom.CounterOpts{
			Name: "flux_http_total",
			Help: "Number of HTTP request/response exchanges",
		}, httpLabels),

		httpRoundtrip: prom.NewSummaryVec(prom.SummaryOpts{
			Name: "flux_http_roundtrip_usec",
			Help: "HTTP response roundtrip time in microseconds",
		}, httpLabels),

		httpTotal: prom.NewSummaryVec(prom.SummaryOpts{
			Name: "flux_http_total_usec",
			Help: "HTTP total response time in microseconds",
		}, httpLabels),
	}

	return h, daemon.Aggregate(h.listenStartFunc,
		cf.advertiseStartFunc()), nil
}

func (h *eventHandler) listenStartFunc(errs daemon.ErrorSink) daemon.Component {
	stopped := false

	for _, c := range h.collectors() {
		errs.Post(prom.Register(c))
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prom.Handler())

	listener, err := net.Listen("tcp", h.listenAddr)
	errs.Post(err)
	if err == nil {
		go func() {
			err := http.Serve(listener, mux)
			if !stopped {
				errs.Post(err)
			}
		}()
	}

	return daemon.StopFunc(func() {
		stopped = true

		if listener != nil {
			errs.Post(listener.Close())
		}

		for _, c := range h.collectors() {
			prom.Unregister(c)
		}
	})
}

func (h *eventHandler) collectors() []prom.Collector {
	return []prom.Collector{h.connections, h.http, h.httpRoundtrip,
		h.httpTotal}
}

func (h *eventHandler) Connection(ev *events.Connection) {
	h.connections.WithLabelValues(ev.InstanceName, ev.Inbound.IP.String(), ev.InstanceAddr.IP().String(), ev.Protocol).Inc()
}

func (h *eventHandler) HttpExchange(ev *events.HttpExchange) {
	src := ev.Inbound.IP.String()
	dst := ev.InstanceAddr.IP().String()
	method := ev.Request.Method
	code := strconv.Itoa(ev.Response.StatusCode)
	h.http.WithLabelValues(ev.InstanceName, src, dst, method, code).Inc()
	h.httpRoundtrip.WithLabelValues(ev.InstanceName, src, dst, method, code).Observe(float64(ev.RoundTrip / time.Microsecond))
	h.httpTotal.WithLabelValues(ev.InstanceName, src, dst, method, code).Observe(float64(ev.TotalTime / time.Microsecond))
}

const TTL = 5 * time.Minute

func (cf *eventHandlerConfig) advertiseStartFunc() daemon.StartFunc {
	run := func(stop <-chan struct{}, errs daemon.ErrorSink) {
		ctx := context.Background()

		resp, err := cf.etcdClient.CreateInOrder(ctx,
			"/weave-flux/prometheus-targets", cf.advertiseAddr,
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
			case <-stop:
				return
			case <-t.C:
			}

			_, err := cf.etcdClient.Set(ctx, key,
				cf.advertiseAddr, &etcd.SetOptions{TTL: TTL})
			if err != nil {
				errs.Post(err)
				return
			}
		}
	}

	return daemon.Restart(TTL/10, daemon.SimpleComponent(run))
}
