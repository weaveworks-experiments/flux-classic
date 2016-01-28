package events

import (
	"net"
	"net/http"
	"time"

	"github.com/weaveworks/flux/balancer/model"
)

type Handler interface {
	Connection(*Connection)
	HttpExchange(*HttpExchange)

	// Fully activate the handler
	Start()

	// Release any resources
	Stop()
}

type Connection struct {
	Service  *model.Service
	Instance *model.Instance
	Inbound  *net.TCPAddr
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

func (NullHandler) Start() {}
func (NullHandler) Stop()  {}
