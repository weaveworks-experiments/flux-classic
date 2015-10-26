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

	"github.com/dpw/ambergris/interceptor/events"

	"github.com/stretchr/testify/require"
)

type shimHarness struct {
	listener    *net.TCPListener
	exchanges   []*events.HttpExchange
	connections int
	events.DiscardOthers
}

func wrapShim(shim shimFunc, target *net.TCPAddr, check func(error)) *shimHarness {
	listener, err := net.ListenTCP("tcp", nil)
	check(err)

	h := &shimHarness{listener: listener}

	go func() {
		for {
			inbound, err := listener.AcceptTCP()
			check(err)
			h.connections++
			go func() {
				outbound, err := net.DialTCP("tcp", nil, target)
				check(err)
				check(shim(inbound, outbound, h))
			}()
		}
	}()

	return h
}

func (h *shimHarness) addr() *net.TCPAddr {
	return h.listener.Addr().(*net.TCPAddr)
}

func (h *shimHarness) stop() error {
	return h.listener.Close()
}

func (h *shimHarness) HttpExchange(exch *events.HttpExchange) {
	h.exchanges = append(h.exchanges, exch)
}

func TestHttp(t *testing.T) {
	check := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randStr := func() string {
		return fmt.Sprint(r.Int63())
	}

	read := func(r io.ReadCloser) string {
		b, err := ioutil.ReadAll(r)
		check(err)
		check(r.Close())
		return string(b)
	}

	var expectOut, gotIn string

	mux := http.NewServeMux()
	mux.HandleFunc("/out", func(w http.ResponseWriter, req *http.Request) {
		w.Write(([]byte)(expectOut))
	})
	mux.HandleFunc("/in", func(w http.ResponseWriter, req *http.Request) {
		gotIn = read(req.Body)
	})
	mux.HandleFunc("/inout", func(w http.ResponseWriter, req *http.Request) {
		gotIn = read(req.Body)
		w.Write(([]byte)(expectOut))
	})

	l, err := net.ListenTCP("tcp", nil)
	check(err)
	go func() { http.Serve(l, mux) }()

	harness := wrapShim(httpShim, l.Addr().(*net.TCPAddr), check)
	url := fmt.Sprintf("http://localhost:%d/", harness.addr().Port)

	doGet := func() string {
		res, err := http.Get(url + "out")
		check(err)
		return read(res.Body)
	}

	doPost := func(s string) {
		_, err := http.Post(url+"in", "text/plain",
			bytes.NewBuffer(([]byte)(s)))
		check(err)
	}

	doPostInOut := func(s string) string {
		res, err := http.Post(url+"inout", "text/plain",
			bytes.NewBuffer(([]byte)(s)))
		check(err)
		return read(res.Body)
	}

	expectOut = randStr()
	require.Equal(t, doGet(), expectOut)
	require.Equal(t, "GET", harness.exchanges[0].Request.Method)
	require.Equal(t, "/out", harness.exchanges[0].Request.URL.String())
	require.Equal(t, 200, harness.exchanges[0].Response.StatusCode)
	require.True(t, harness.exchanges[0].RoundTrip > 0*time.Second && harness.exchanges[0].RoundTrip < 100*time.Millisecond)
	require.True(t, harness.exchanges[0].TotalTime > 0*time.Second && harness.exchanges[0].TotalTime < 100*time.Millisecond)

	expectIn := randStr()
	doPost(expectIn)
	require.Equal(t, gotIn, expectIn)

	expectIn = randStr()
	require.Equal(t, doPostInOut(expectIn), expectOut)
	require.Equal(t, gotIn, expectIn)

	expectOut = randStr()
	require.Equal(t, doGet(), expectOut)

	expectIn = randStr()
	doPost(expectIn)
	require.Equal(t, gotIn, expectIn)

	expectIn = randStr()
	require.Equal(t, doPostInOut(expectIn), expectOut)
	require.Equal(t, gotIn, expectIn)

	require.Len(t, harness.exchanges, 6)

	// should have re-used one connection for all requests
	require.Equal(t, 1, harness.connections)

	check(l.Close())
	check(harness.stop())
}
