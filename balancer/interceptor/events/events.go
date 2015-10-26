package events

import (
	"net"
	"net/http"
	"time"
)

type Handler interface {
	Connection(*Connection)
	HttpExchange(*HttpExchange)
}

type Connection struct {
	Inbound  *net.TCPAddr
	Outbound *net.TCPAddr
	Protocol string
}

type HttpExchange struct {
	Inbound   *net.TCPAddr
	Outbound  *net.TCPAddr
	Request   *http.Request
	Response  *http.Response
	RoundTrip time.Duration
	TotalTime time.Duration
}

type DiscardOthers struct{}

func (DiscardOthers) Connection(*Connection) {}

func (DiscardOthers) HttpExchange(*HttpExchange) {}
