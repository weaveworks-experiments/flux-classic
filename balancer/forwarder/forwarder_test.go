package forwarder

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
)

func TestForwarder(t *testing.T) {
	listener, err := net.ListenTCP("tcp", nil)
	require.Nil(t, err)
	laddr := listener.Addr().(*net.TCPAddr)

	errorSink := daemon.NewErrorSink()
	fwd, err := Config{
		ServiceName:  "service",
		Description:  "service",
		BindIP:       net.ParseIP("127.42.0.1"),
		EventHandler: events.NullHandler{},
		ErrorSink:    errorSink,
	}.New()
	require.Nil(t, err)

	fwd.SetProtocol("tcp")
	fwd.SetInstances(map[string]netutil.IPPort{
		"inst": netutil.NewIPPort(laddr.IP, laddr.Port),
	})

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	expect := fmt.Sprint(rng.Int63())
	got := ""

	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				return
			}

			b, err := ioutil.ReadAll(conn)
			require.Nil(t, err)
			require.Nil(t, conn.Close())
			got = string(b)
		}
	}()

	require.Nil(t, err)
	conn, err := net.DialTCP("tcp", nil, fwd.Addr())
	require.Nil(t, err)
	_, err = conn.Write([]byte(expect))
	require.Nil(t, err)
	require.Nil(t, conn.CloseWrite())
	_, err = ioutil.ReadAll(conn)
	require.Nil(t, err)
	require.Nil(t, conn.Close())
	require.Equal(t, expect, got)

	listener.Close()
	fwd.Stop()
}
