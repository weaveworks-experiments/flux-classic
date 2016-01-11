package events

import (
	"net"
	"net/http"
	"time"

	"github.com/squaremo/flux/balancer/model"
)

type Handler interface {
	Connection(*Connection)
	HttpExchange(*HttpExchange)
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
