package balancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"math/rand"
	"net"
	"sync"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/common/errorsink"
)

type forwardingConfig struct {
	netConfig
	key model.ServiceKey
	*ipTables
	eventHandler events.Handler
	errorSink    errorsink.ErrorSink
}

type forwarding struct {
	forwardingConfig
	rule     []interface{}
	listener *net.TCPListener
	stopped  bool

	lock sync.Mutex
	*model.ServiceInfo
	shim     shimFunc
	shimName string
}

type shimFunc func(inbound, outbound *net.TCPConn, conn *events.Connection, eventHandler events.Handler) error

func (fc forwardingConfig) start(si *model.ServiceInfo) (serviceState, error) {
	ip, err := bridgeIP(fc.bridge)
	if err != nil {
		return nil, err
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: ip})
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
		"-d", fc.key.IP(),
		"--dport", fc.key.Port,
		"-j", "DNAT",
		"--to-destination", listener.Addr(),
	}
	err = fc.ipTables.addRule("nat", rule)
	if err != nil {
		return nil, err
	}

	fwd := &forwarding{
		forwardingConfig: fc,
		rule:             rule,
		listener:         listener,
		ServiceInfo:      si,
	}

	fwd.chooseShim()
	go fwd.run()
	success = true
	return fwd, nil
}

func bridgeIP(br string) (net.IP, error) {
	iface, err := net.InterfaceByName(br)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		if cidr, ok := addr.(*net.IPNet); ok {
			if ip := cidr.IP.To4(); ip != nil {
				return ip, nil
			}
		}
	}

	return nil, fmt.Errorf("no IPv4 address found on netdev %s", br)
}

func (fwd *forwarding) run() {
	for {
		conn, err := fwd.listener.AcceptTCP()
		if err != nil {
			if !fwd.stopped {
				fwd.errorSink.Post(err)
			}
			return
		}

		go fwd.forward(conn)
	}
}

func (fwd *forwarding) stop() {
	fwd.stopped = true
	fwd.listener.Close()
	fwd.ipTables.deleteRule("nat", fwd.rule)
}

func (fwd *forwarding) update(si *model.ServiceInfo) (bool, error) {
	if len(si.Instances) > 0 {
		fwd.lock.Lock()
		defer fwd.lock.Unlock()
		fwd.ServiceInfo = si
		fwd.chooseShim()
		return true, nil
	}

	return false, nil
}

var shims = map[string]shimFunc{
	"tcp":  tcpShim,
	"http": httpShim,
}

func (fwd *forwarding) chooseShim() {
	name := fwd.Protocol
	if name == "" {
		name = "tcp"
	}

	shim := shims[name]
	if shim == nil {
		log.Warn("service ", fwd.key, ": no support for protocol ",
			fwd.Protocol, ", falling back to TCP forwarding")
		shim = tcpShim
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
	err = shim(inbound, outbound, connEvent, fwd.eventHandler)
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
