package balancer

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/common/errorsink"
)

// Test that forward.go plugs everything together correctly, and
// exercise the tcp shim.
func TestForward(t *testing.T) {
	nc := netConfig{
		chain:  "AMBERGREEN",
		bridge: "lo",
	}

	mipt := newMockIPTables(t)
	ipTables := newIPTables(nc, mipt.cmd)
	ipTables.start()

	listener, err := net.ListenTCP("tcp", nil)
	require.Nil(t, err)
	laddr := listener.Addr().(*net.TCPAddr)

	errorSink := errorsink.New()
	key := model.MakeServiceKey("tcp", net.ParseIP("127.42.0.1"), 8888)
	ss, err := forwardingConfig{
		netConfig:    nc,
		key:          key,
		ipTables:     ipTables,
		eventHandler: events.DiscardOthers{},
		errorSink:    errorSink,
	}.start(&model.ServiceInfo{Instances: []model.Instance{
		model.MakeInstance("foo", "bar", laddr.IP, laddr.Port),
	}})
	require.Nil(t, err)

	require.Len(t, mipt.chains["nat AMBERGREEN"], 1)
	rule := mipt.chains["nat AMBERGREEN"][0]
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(rule, " "))

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

	faddr, err := net.ResolveTCPAddr("tcp", rule[len(rule)-1])
	conn, err := net.DialTCP("tcp", nil, faddr)
	require.Nil(t, err)
	_, err = conn.Write([]byte(expect))
	require.Nil(t, err)
	require.Nil(t, conn.CloseWrite())
	_, err = ioutil.ReadAll(conn)
	require.Nil(t, err)
	require.Nil(t, conn.Close())
	require.Equal(t, expect, got)

	listener.Close()
	ss.stop()
}
