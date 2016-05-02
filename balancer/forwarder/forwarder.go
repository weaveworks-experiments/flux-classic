package forwarder

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"net"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/pool"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
)

const max_connection_attempts = 5

type Config struct {
	ServiceName  string
	Description  string // for logging
	BindIP       net.IP
	EventHandler events.Handler
	ErrorSink    daemon.ErrorSink
}

type Forwarder struct {
	Config

	listener *net.TCPListener
	pool     *pool.InstancePool
	protocol string
	shim     shimFunc
	stopped  bool
}

type shimFunc func(inbound, outbound *net.TCPConn, conn *events.Connection, eventHandler events.Handler) error

func (cf Config) New() (*Forwarder, error) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: cf.BindIP})
	if err != nil {
		return nil, err
	}

	fwd := &Forwarder{
		Config:   cf,
		listener: listener,
		pool:     pool.NewInstancePool(),
		protocol: "tcp",
		shim:     tcpShim,
	}

	go fwd.run()
	return fwd, nil
}

func (fwd *Forwarder) Addr() *net.TCPAddr {
	return fwd.listener.Addr().(*net.TCPAddr)
}

func (fwd *Forwarder) Stop() {
	fwd.stopped = true
	fwd.listener.Close()
	fwd.pool.Stop()
}

func (fwd *Forwarder) run() {
	for {
		conn, err := fwd.listener.AcceptTCP()
		if err != nil {
			if !fwd.stopped {
				fwd.ErrorSink.Post(err)
			}
			return
		}

		go fwd.forward(conn)
	}
}

func (fwd *Forwarder) forward(inbound *net.TCPConn) {
	inAddr := inbound.RemoteAddr().(*net.TCPAddr)

	for i := 0; i < max_connection_attempts; i++ {
		inst := fwd.pool.PickInstance()
		if inst == nil {
			log.Errorf("%s: ran out of instances for connection from %s",
				fwd.Description, inAddr)
			return
		}

		outbound, err := net.DialTCP("tcp", nil, inst.Address.TCPAddr())
		if err != nil {
			log.Errorf("%s: connecting to %s: %s",
				fwd.Description, inst.Address, err)
			fwd.pool.Failed(inst)
			continue
		}

		fwd.pool.Succeeded(inst)
		connEvent := &events.Connection{
			ServiceName:  fwd.ServiceName,
			Protocol:     fwd.protocol,
			InstanceName: inst.Name,
			InstanceAddr: inst.Address,
			Inbound:      inAddr,
		}
		fwd.EventHandler.Connection(connEvent)

		err = fwd.shim(inbound, outbound, connEvent, fwd.EventHandler)
		if err != nil {
			log.Errorf("%s: forwarding from %s to %s: %s",
				fwd.Description, inAddr, inst.Address, err)
		}
		return
	}

	inbound.Close()
	log.Errorf("%s: gave up trying to connect for connection from %s",
		fwd.Description, inAddr)
}

var shims = map[string]shimFunc{
	"tcp":  tcpShim,
	"http": httpShim,
}

func (fwd *Forwarder) SetProtocol(proto string) {
	shim := shims[proto]
	if shim == nil {
		log.Warn(fwd.Description,
			": no support for protocol ", proto,
			", falling back to TCP forwarding")
		proto = "tcp"
		shim = tcpShim
	}

	fwd.protocol = proto
	fwd.shim = shim
}

func (fwd *Forwarder) SetInstances(instances map[string]netutil.IPPort) {
	fwd.pool.UpdateInstances(instances)
}

func tcpShim(inbound, outbound *net.TCPConn, connEvent *events.Connection, eh events.Handler) error {
	ch := make(chan error, 1)
	go func() {
		var err error
		defer func() { ch <- err }()
		_, err = io.Copy(inbound, outbound)
		outbound.CloseRead()
		inbound.CloseWrite()
	}()

	_, err1 := io.Copy(outbound, inbound)
	inbound.CloseRead()
	outbound.CloseWrite()

	err2 := <-ch
	inbound.Close()
	outbound.Close()

	if err1 != nil {
		return err1
	} else {
		return err2
	}
}
