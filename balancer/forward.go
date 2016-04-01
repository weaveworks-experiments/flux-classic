package balancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"net"
	"sync"
	"time"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/balancer/pool"
	"github.com/weaveworks/flux/common/daemon"
)

const max_connection_attempts = 5

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

	lock    sync.Mutex
	service *model.Service

	pool        pool.InstancePool
	retryTicker *time.Ticker

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
		service:          svc,
		pool:             pool.NewInstancePool(),
		retryTicker:      time.NewTicker(1 * time.Second),
	}
	fwd.pool.UpdateInstances(svc.Instances)
	go func() {
		for {
			t := <-fwd.retryTicker.C
			if t.IsZero() {
				return
			}
			fwd.lock.Lock()
			fwd.pool.ReactivateRetries(t)
			fwd.lock.Unlock()
		}
	}()

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

	// same address and same set of instances; stay as it is
	if svc.Equal(fwd.service) {
		return true, nil
	}

	if !svc.IP.Equal(fwd.service.IP) || svc.Port != fwd.service.Port {
		return false, nil
	}

	log.Info("forwarding service: ", svc.Summary())

	fwd.pool.UpdateInstances(svc.Instances)
	fwd.service = svc
	fwd.chooseShim()
	return true, nil
}

var shims = map[string]shimFunc{
	"tcp":  tcpShim,
	"http": httpShim,
}

func (fwd *forwarding) chooseShim() {
	shim := shims[fwd.service.Protocol]
	if shim == nil {
		log.Warn("service ", fwd.service.Name,
			": no support for protocol ", fwd.service.Protocol,
			", falling back to TCP forwarding")
		shim = tcpShim
	}

	fwd.shim = shim
}

func (fwd *forwarding) forward(inbound *net.TCPConn) {
	inAddr := inbound.RemoteAddr().(*net.TCPAddr)

	for i := 0; i < max_connection_attempts; i++ {
		inst, shim := fwd.pickInstanceAndShim()
		if inst == nil {
			log.Errorf("ran out of instances attempting connection %s->%s (%s)",
				inAddr, fwd.service.TCPAddr(), fwd.service.Name)
			return
		}
		outAddr := inst.Instance().TCPAddr()

		outbound, err := net.DialTCP("tcp", nil, outAddr)
		if err != nil {
			log.Error("connecting to ", outAddr, ": ", err)
			inst.Fail()
			continue
		}
		inst.Keep()

		connEvent := &events.Connection{
			Service:  fwd.service,
			Instance: inst.Instance(),
			Inbound:  inAddr,
		}
		err = shim(inbound, outbound, connEvent, fwd.eventHandler)
		if err != nil {
			log.Error("forwarding from ", inAddr, " to ", outAddr, ": ",
				err)
		}
		return
	}
	inbound.Close()
	log.Errorf("abandoned connection %s->%s (%s) after reaching max of %d attempts",
		inAddr, fwd.service.TCPAddr(), fwd.service.Name, max_connection_attempts)
}

func (fwd *forwarding) pickInstanceAndShim() (pool.PooledInstance, shimFunc) {
	fwd.lock.Lock()
	defer fwd.lock.Unlock()
	inst := fwd.pool.PickInstance()
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
