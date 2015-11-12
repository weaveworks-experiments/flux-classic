package simplecontrol

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/squaremo/ambergreen/balancer/interceptor/model"
)

// A simple control mechanism for the daemon via a unix socket.
// Intended to prove out the main interceptor code and for testing
// rather than as something to be used in anger.

type Server struct {
	errors   chan<- error
	listener *net.UnixListener
	updates  chan model.ServiceUpdate
	lock     sync.Mutex
	closed   chan struct{}
	finished chan struct{}
}

const SOCKET = "/var/run/ambergris.sock"

func NewServer(errors chan<- error) (*Server, error) {
	os.Remove(SOCKET)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{SOCKET, "unix"})
	if err != nil {
		return nil, err
	}

	srv := &Server{
		listener: listener,
		updates:  make(chan model.ServiceUpdate),
		closed:   make(chan struct{}),
		finished: make(chan struct{}),
	}
	go srv.run(listener)
	return srv, nil
}

func (srv *Server) Updates() <-chan model.ServiceUpdate {
	return srv.updates
}

func (srv *Server) Close() {
	srv.lock.Lock()
	defer srv.lock.Unlock()

	if srv.listener != nil {
		srv.listener.Close()
		close(srv.closed)
		srv.listener = nil
		<-srv.finished
	}
}

func (srv *Server) run(listener *net.UnixListener) {
	defer os.Remove(SOCKET)

	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			select {
			case srv.errors <- err:
			case <-srv.closed:
			}
			break
		}

		go srv.handleConn(conn)
	}

	close(srv.finished)
}

func (srv *Server) handleConn(conn *net.UnixConn) {
	err := srv.doRequest(conn)
	resp := "Ok\n"
	if err != nil {
		resp = err.Error()
	}

	conn.Write(([]byte)(resp))
	conn.Close()
}

func (srv *Server) doRequest(conn *net.UnixConn) error {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, conn)
	if err != nil {
		return err
	}

	// XXX support service deletion commands

	parts := strings.Split(strings.TrimSpace(buf.String()), " ")
	if len(parts) <= 0 {
		return fmt.Errorf("service specification should begin with port:ip-address")
	}

	addr, err := net.ResolveTCPAddr("tcp", parts[0])
	if err != nil {
		return err
	}

	var insts []model.Instance
	for _, inst := range parts[2:] {
		addr, err := net.ResolveTCPAddr("tcp", inst)
		if err != nil {
			return err
		}
		insts = append(insts, model.MakeInstance(inst, "default", addr.IP, addr.Port))
	}

	var update model.ServiceUpdate
	update.ServiceKey = model.MakeServiceKey("tcp", addr.IP, addr.Port)
	update.ServiceInfo = &model.ServiceInfo{
		Protocol:  parts[1],
		Instances: insts,
	}

	select {
	case srv.updates <- update:
	case <-srv.closed:
	}

	return nil
}
