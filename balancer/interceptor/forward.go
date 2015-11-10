package interceptor

import (
	log "github.com/Sirupsen/logrus"
	"io"
	"math/rand"
	"net"
	"sync"

	"github.com/squaremo/ambergreen/balancer/interceptor/events"
	"github.com/squaremo/ambergreen/balancer/interceptor/model"
)

type forwarding struct {
	*service
	rule     []interface{}
	listener *net.TCPListener
	stopCh   chan struct{}

	lock sync.Mutex
	*model.ServiceInfo
	shim     shimFunc
	shimName string
}

type shimFunc func(inbound, outbound *net.TCPConn, conn *events.Connection, eventHandler events.Handler) error

func (svc *service) startForwarding(upd model.ServiceUpdate) (serviceState, error) {
	bridgeIP, err := svc.config.bridgeIP()
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: bridgeIP})
	if err != nil {
		return nil, err
	}

	success := false
	defer func() {
		if !success {
			listener.Close()
		}
	}()

	rule := []interface{}{
		"-p", "tcp",
		"-d", upd.IP(),
		"--dport", upd.Port,
		"-j", "DNAT",
		"--to-destination", listener.Addr(),
	}
	err = svc.config.addRule("nat", rule)
	if err != nil {
		return nil, err
	}

	fwd := &forwarding{
		service:     svc,
		rule:        rule,
		listener:    listener,
		stopCh:      make(chan struct{}),
		ServiceInfo: upd.ServiceInfo,
	}

	fwd.chooseShim()
	go fwd.run()
	success = true
	return fwd, nil
}

func (fwd *forwarding) run() {
	for {
		conn, err := fwd.listener.AcceptTCP()
		if err != nil {
			// Seems like a fatal error
			select {
			case fwd.errors <- err:
			case <-fwd.stopCh:
			}
			return
		}

		go fwd.forward(conn)
	}
}

func (fwd *forwarding) stop() {
	fwd.listener.Close()
	close(fwd.stopCh)
	fwd.config.deleteRule("nat", fwd.rule)
}

func (fwd *forwarding) update(upd model.ServiceUpdate) (bool, error) {
	if len(upd.Instances) > 0 {
		fwd.lock.Lock()
		defer fwd.lock.Unlock()
		fwd.ServiceInfo = upd.ServiceInfo
		fwd.chooseShim()
		return true, nil
	}

	return false, nil
}

func (fwd *forwarding) chooseShim() {
	name := fwd.Protocol
	shim := tcpShim

	switch fwd.Protocol {
	case "", "tcp":
		name = "tcp"

	case "http":
		shim = httpShim

	default:
		log.Warn("service ", fwd.key, ": no support for protocol ",
			fwd.Protocol, ", falling back to TCP forwarding")
		name = "tcp"
	}

	fwd.shim = shim
	fwd.shimName = name
}

func (fwd *forwarding) forward(inbound *net.TCPConn) {
	inst, shim, shimName := fwd.pickInstanceAndShim()
	inAddr := inbound.RemoteAddr().(*net.TCPAddr)
	outAddr := inst.TCPAddr()

	outbound, err := net.DialTCP("tcp", nil, outAddr)
	if err != nil {
		log.Error("connecting to ", outAddr, ": ", err)
		return
	}

	connEvent := &events.Connection{
		Ident:    inst.Ident,
		Inbound:  inAddr,
		Outbound: outAddr,
		Protocol: shimName,
	}
	err = shim(inbound, outbound, connEvent, fwd.config.eventHandler)
	if err != nil {
		log.Error("forwarding from ", inAddr, " to ", outAddr, ": ",
			err)
	}
}

func (fwd *forwarding) pickInstanceAndShim() (model.Instance, shimFunc, string) {
	fwd.lock.Lock()
	defer fwd.lock.Unlock()
	return fwd.Instances[rand.Intn(len(fwd.Instances))], fwd.shim, fwd.shimName
}

func tcpShim(inbound, outbound *net.TCPConn, connEvent *events.Connection, eh events.Handler) error {
	eh.Connection(connEvent)
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
