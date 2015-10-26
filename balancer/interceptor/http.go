package interceptor

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/squaremo/ambergreen/balancer/interceptor/events"
)

func httpShim(inbound, outbound *net.TCPConn, eh events.Handler) error {
	reqrd := bufio.NewReader(inbound)
	resprd := bufio.NewReader(outbound)
	defer inbound.Close()
	defer outbound.Close()

	inboundAddr := inbound.RemoteAddr().(*net.TCPAddr)
	outboundAddr := outbound.RemoteAddr().(*net.TCPAddr)

	for {
		// XXX timeout on no request
		req, err := http.ReadRequest(reqrd)
		t1 := time.Now()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		req.Write(outbound)
		resp, err := http.ReadResponse(resprd, req)
		t2 := time.Now()
		if err != nil {
			return err
		}

		resp.Write(inbound)
		t3 := time.Now()

		eh.HttpExchange(&events.HttpExchange{
			Inbound:   inboundAddr,
			Outbound:  outboundAddr,
			Request:   req,
			Response:  resp,
			RoundTrip: t2.Sub(t1),
			TotalTime: t3.Sub(t1),
		})
	}
}
