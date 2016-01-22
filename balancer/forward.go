package balancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"math/rand"
	"net"
	"sync"

	"github.com/squaremo/flux/balancer/events"
	"github.com/squaremo/flux/balancer/model"
	"github.com/squaremo/flux/common/daemon"
)

type forwardingConfig struct {
	netConfig
	*ipTables
	eventHandler events.Handler
	errorSink    daemon.ErrorSink
}

type forwarding struct {
	forwardingConfig
	rule     []interface{}
	listener *net.TCPListener
	stopped  bool

	lock sync.Mutex
	*model.Service
	shim shimFunc
}

type shimFunc func(inbound, outbound *net.TCPConn, conn *events.Connection, eventHandler events.Handler) error

func (fc forwardingConfig) start(svc *model.Service) (serviceState, error) {
	log.Info("forwarding service: ", svc.Summary())
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
		"-d", svc.IP,
		"--dport", svc.Port,
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
		Service:          svc,
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

func (fwd *forwarding) update(svc *model.Service) (bool, error) {
	if len(svc.Instances) == 0 {
		return false, nil
	}

	fwd.lock.Lock()
	defer fwd.lock.Unlock()

	if svc.Equal(fwd.Service) {
		return true, nil
	}

	if !svc.IP.Equal(fwd.Service.IP) || svc.Port != fwd.Service.Port {
		return false, nil
	}

	log.Info("forwarding service: ", svc.Summary())
	fwd.Service = svc
	fwd.chooseShim()
	return true, nil
}

var shims = map[string]shimFunc{
	"tcp":  tcpShim,
	"http": httpShim,
}

func (fwd *forwarding) chooseShim() {
	shim := shims[fwd.Protocol]
	if shim == nil {
		log.Warn("service ", fwd.Service.Name,
			": no support for protocol ", fwd.Protocol,
			", falling back to TCP forwarding")
		shim = tcpShim
	}

	fwd.shim = shim
}

func (fwd *forwarding) forward(inbound *net.TCPConn) {
	inst, shim := fwd.pickInstanceAndShim()
	inAddr := inbound.RemoteAddr().(*net.TCPAddr)
	outAddr := inst.TCPAddr()

	outbound, err := net.DialTCP("tcp", nil, outAddr)
	if err != nil {
		log.Error("connecting to ", outAddr, ": ", err)
		return
	}

	connEvent := &events.Connection{
		Service:  fwd.Service,
		Instance: inst,
		Inbound:  inAddr,
	}
	err = shim(inbound, outbound, connEvent, fwd.eventHandler)
	if err != nil {
		log.Error("forwarding from ", inAddr, " to ", outAddr, ": ",
			err)
	}
}

func (fwd *forwarding) pickInstanceAndShim() (*model.Instance, shimFunc) {
	fwd.lock.Lock()
	defer fwd.lock.Unlock()
	inst := &fwd.Instances[rand.Intn(len(fwd.Instances))]
	return inst, fwd.shim
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
