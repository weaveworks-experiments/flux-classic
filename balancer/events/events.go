package events

import (
	"net"
	"net/http"
	"time"

	"github.com/weaveworks/flux/common/netutil"
)

type Handler interface {
	Connection(*Connection)
	HttpExchange(*HttpExchange)

	// Release any resources
	Stop()
}

type Connection struct {
	ServiceName  string
	Protocol     string
	InstanceName string
	InstanceAddr netutil.IPPort
	Inbound      *net.TCPAddr
}

type HttpExchange struct {
	*Connection
	Request   *http.Request
	Response  *http.Response
	RoundTrip time.Duration
	TotalTime time.Duration
}

type DiscardOthers struct{}

func (DiscardOthers) Connection(*Connection) {}

func (DiscardOthers) HttpExchange(*HttpExchange) {}

type NullHandler struct{ DiscardOthers }

func (NullHandler) Stop() {}
