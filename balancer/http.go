package balancer

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/squaremo/ambergreen/balancer/events"
)

func httpShim(inbound, outbound *net.TCPConn, connEvent *events.Connection, eh events.Handler) error {
	eh.Connection(connEvent)
	defer inbound.Close()
	defer outbound.Close()

	// Request handling and response handling take place in
	// separate goroutines.  This is not to support pipelining
	// (although it could easily be extended to do so).  Rather,
	// it is to support cases where the server produces a response
	// before the client is done sending the request (e.g. when
	// the server produces an error response before reading the
	// whole request).

	type request struct {
		req      *http.Request
		tReadReq time.Time
		err      error
	}

	reqCh := make(chan request)
	doneCh := make(chan struct{})
	defer close(doneCh)

	passReq := func(req request) bool {
		select {
		case reqCh <- req:
			return true
		case <-doneCh:
			return false
		}
	}

	// The request handler loop
	go func() {
		reqrd := bufio.NewReader(inbound)
		for {
			// XXX timeout on no request
			req, err := http.ReadRequest(reqrd)
			if err != nil {
				if err == io.EOF {
					close(reqCh)
					return
				}

				passReq(request{err: err})
				return
			}

			if !passReq(request{req: req, tReadReq: time.Now()}) {
				return
			}

			err = req.Write(outbound)
			if err != nil {
				passReq(request{err: err})
				return
			}
		}
	}()

	// The response handler loop
	resprd := bufio.NewReader(outbound)
	for {
		req, ok := <-reqCh
		if !ok {
			return nil
		}

		if req.err != nil {
			return req.err
		}

		resp, err := http.ReadResponse(resprd, req.req)
		tReadResponse := time.Now()
		if err != nil {
			return err
		}

		err = resp.Write(inbound)
		tWroteResponse := time.Now()
		if err != nil {
			return err
		}

		eh.HttpExchange(&events.HttpExchange{
			Connection: connEvent,
			Request:    req.req,
			Response:   resp,
			RoundTrip:  tReadResponse.Sub(req.tReadReq),
			TotalTime:  tWroteResponse.Sub(req.tReadReq),
		})
	}
}
