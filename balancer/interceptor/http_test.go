package interceptor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/squaremo/ambergreen/balancer/interceptor/events"
	"github.com/squaremo/ambergreen/balancer/interceptor/model"

	"github.com/stretchr/testify/require"
)

type shimWrapper struct {
	listener    *net.TCPListener
	baseUrl     string
	exchanges   chan *events.HttpExchange
	connections int
	events.DiscardOthers
}

func wrapShim(shim shimFunc, target *net.TCPAddr, t *testing.T) *shimWrapper {
	listener, err := net.ListenTCP("tcp", nil)
	require.Nil(t, err)

	w := &shimWrapper{
		listener:  listener,
		exchanges: make(chan *events.HttpExchange, 100),
		baseUrl: fmt.Sprintf("http://localhost:%d/",
			listener.Addr().(*net.TCPAddr).Port),
	}

	go func() {
		for {
			inbound, err := listener.AcceptTCP()
			if w.listener == nil {
				return
			}

			require.Nil(t, err)

			w.connections++
			go func() {
				outbound, err := net.DialTCP("tcp", nil, target)
				require.Nil(t, err)
				cevent := &events.Connection{
					Ident:    model.Ident{target.String(), "default"},
					Inbound:  inbound.RemoteAddr().(*net.TCPAddr),
					Outbound: target,
					Protocol: "http",
				}
				require.Nil(t, shim(inbound, outbound, cevent, w))
			}()
		}
	}()

	return w
}

func (w *shimWrapper) addr() *net.TCPAddr {
	return w.listener.Addr().(*net.TCPAddr)
}

func (w *shimWrapper) stop() error {
	l := w.listener
	w.listener = nil
	return l.Close()
}

func (w *shimWrapper) HttpExchange(exch *events.HttpExchange) {
	w.exchanges <- exch
}

func readAll(r io.ReadCloser, t *testing.T) string {
	b, err := ioutil.ReadAll(r)
	require.Nil(t, err)
	require.Nil(t, r.Close())
	return string(b)
}

type harness struct {
	server           net.Listener
	expectOut, gotIn string
	*shimWrapper
}

func newHarness(t *testing.T) *harness {
	l, err := net.ListenTCP("tcp", nil)
	require.Nil(t, err)

	h := &harness{server: l}

	mux := http.NewServeMux()
	mux.HandleFunc("/out", func(w http.ResponseWriter, req *http.Request) {
		w.Write(([]byte)(h.expectOut))
	})
	mux.HandleFunc("/in", func(w http.ResponseWriter, req *http.Request) {
		h.gotIn = readAll(req.Body, t)
	})
	mux.HandleFunc("/inout", func(w http.ResponseWriter, req *http.Request) {
		h.gotIn = readAll(req.Body, t)
		w.Write(([]byte)(h.expectOut))
	})

	h.shimWrapper = wrapShim(httpShim, l.Addr().(*net.TCPAddr), t)

	go func() { http.Serve(l, mux) }()

	return h
}

func (h *harness) stop(t *testing.T) {
	require.Nil(t, h.server.Close())
	require.Nil(t, h.shimWrapper.stop())
}

func (h *harness) get(c *http.Client, t *testing.T) string {
	res, err := c.Get(h.baseUrl + "out")
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	return readAll(res.Body, t)
}

func (h *harness) post(c *http.Client, s string, t *testing.T) {
	res, err := c.Post(h.baseUrl+"in", "text/plain",
		bytes.NewBuffer(([]byte)(s)))
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
}

func (h *harness) postInOut(c *http.Client, s string, t *testing.T) string {
	res, err := c.Post(h.baseUrl+"inout", "text/plain",
		bytes.NewBuffer(([]byte)(s)))
	require.Nil(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	return readAll(res.Body, t)
}

func randStr(r *rand.Rand) string {
	return fmt.Sprint(r.Int63())
}

func test(harness *harness, client *http.Client, t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	harness.expectOut = randStr(r)
	require.Equal(t, harness.get(client, t), harness.expectOut)
	exch := <-harness.exchanges
	require.Equal(t, "GET", exch.Request.Method)
	require.Equal(t, "/out", exch.Request.URL.String())
	require.Equal(t, 200, exch.Response.StatusCode)
	require.True(t, exch.RoundTrip > 0*time.Second && exch.RoundTrip < 100*time.Millisecond)
	require.True(t, exch.TotalTime > 0*time.Second && exch.TotalTime < 100*time.Millisecond)

	expectIn := randStr(r)
	harness.post(client, expectIn, t)
	require.Equal(t, harness.gotIn, expectIn)
	require.Equal(t, "POST", (<-harness.exchanges).Request.Method)

	expectIn = randStr(r)
	require.Equal(t, harness.postInOut(client, expectIn, t),
		harness.expectOut)
	require.Equal(t, harness.gotIn, expectIn)
	require.Equal(t, "POST", (<-harness.exchanges).Request.Method)

	harness.expectOut = randStr(r)
	require.Equal(t, harness.get(client, t), harness.expectOut)
	require.Equal(t, "GET", (<-harness.exchanges).Request.Method)

	expectIn = randStr(r)
	harness.post(client, expectIn, t)
	require.Equal(t, harness.gotIn, expectIn)
	require.Equal(t, "POST", (<-harness.exchanges).Request.Method)

	expectIn = randStr(r)
	require.Equal(t, harness.postInOut(client, expectIn, t),
		harness.expectOut)
	require.Equal(t, harness.gotIn, expectIn)
	require.Equal(t, "POST", (<-harness.exchanges).Request.Method)
}

func TestHttp(t *testing.T) {
	harness := newHarness(t)
	defer harness.stop(t)
	test(harness, http.DefaultClient, t)
	require.Equal(t, 1, harness.connections)
}

func noKeepAlivesClient() *http.Client {
	transport := *http.DefaultTransport.(*http.Transport)
	transport.DisableKeepAlives = true
	return &http.Client{Transport: &transport}
}

func TestHttpNoKeepAlive(t *testing.T) {
	harness := newHarness(t)
	defer harness.stop(t)

	test(harness, noKeepAlivesClient(), t)
	require.Equal(t, 6, harness.connections)
}
