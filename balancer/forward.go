package balancer

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"net"

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

	service *model.Service
	pool    *pool.InstancePool
	shim    shimFunc
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
		"-d", svc.Address.IP,
		"--dport", svc.Address.Port,
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
	}
	fwd.pool.UpdateInstances(svc.Instances)
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
	fwd.pool.Stop()
	fwd.ipTables.deleteRule("nat", fwd.rule)
}

func (fwd *forwarding) update(svc *model.Service) (bool, error) {
	if len(svc.Instances) == 0 {
		return false, nil
	}

	// same address and same set of instances; stay as it is
	if svc.Equal(fwd.service) {
		return true, nil
	}

	if svc.Address == nil || !svc.Address.Equal(*fwd.service.Address) {
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
		inst := fwd.pool.PickInstance()
		if inst == nil {
			log.Errorf("ran out of instances attempting connection %s->%s (%s)",
				inAddr, fwd.service.Address, fwd.service.Name)
			return
		}

		outbound, err := net.DialTCP("tcp", nil, inst.Address.TCPAddr())
		if err != nil {
			log.Error("connecting to ", inst.Address, ": ", err)
			fwd.pool.Failed(inst)
			continue
		}

		fwd.pool.Succeeded(inst)
		connEvent := &events.Connection{
			Service:      fwd.service,
			InstanceName: inst.Name,
			InstanceAddr: inst.Address,
			Inbound:      inAddr,
		}
		err = fwd.shim(inbound, outbound, connEvent, fwd.eventHandler)
		if err != nil {
			log.Error("forwarding from ", inAddr, " to ",
				inst.Address, ": ", err)
		}
		return
	}
	inbound.Close()
	log.Errorf("abandoned connection %s->%s (%s) after reaching max of %d attempts",
		inAddr, fwd.service.Address, fwd.service.Name, max_connection_attempts)
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
