package balancer

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/eventlogger"
	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
	"github.com/weaveworks/flux/common/test/embeddedetcd"
)

func TestEtcdRestart(t *testing.T) {
	server, err := embeddedetcd.NewSimpleEtcd()
	require.Nil(t, err)
	defer func() { require.Nil(t, server.Destroy()) }()

	c, err := etcdutil.NewClient(server.URL())
	require.Nil(t, err)
	st := etcdstore.New(c)

	mipt := newMockIPTables(t)
	done := make(chan model.ServiceUpdate, 10)

	cf := BalancerConfig{
		IPTablesCmd:       mipt.cmd,
		done:              done,
		reconnectInterval: 100 * time.Millisecond,

		netConfig: netConfig{
			chain:  "FLUX",
			bridge: "lo",
		},
		store: st,
		startEventHandler: func(daemon.ErrorSink) events.Handler {
			return eventlogger.EventLogger{}
		},
	}
	start, err := cf.Prepare()
	require.Nil(t, err)
	errs := daemon.NewErrorSink()

	// Start the balancer and wait for it to process the initial
	// empty update
	balancer := start(errs)
	require.True(t, (<-done).Reset)
	require.Empty(t, errs)

	// Add a service and instance, and check that the balancer
	// heard about it
	require.Nil(t, st.AddService("svc", store.Service{
		Address:  &netutil.IPPort{net.ParseIP("127.42.0.1"), 8888},
		Protocol: "tcp",
	}))
	require.False(t, (<-done).Reset)

	// Stop and restart the etcd server
	require.Nil(t, server.Stop())
	time.Sleep(100 * time.Millisecond)
	require.Nil(t, server.Start())
	time.Sleep(200 * time.Millisecond)

	// The reconnection should lead to a reset update
	require.True(t, (<-done).Reset)

	require.Nil(t, st.AddInstance("svc", "inst", store.Instance{
		Address: &netutil.IPPort{net.ParseIP("127.0.0.1"), 10000},
	}))
	require.False(t, (<-done).Reset)

	// Verify that we are forwarding on the service IP
	require.Len(t, mipt.chains["nat FLUX"], 1)
	require.Len(t, mipt.chains["filter FLUX"], 0)
	// NB regexp related to service IP and port given in test case
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat FLUX"][0], " "))

	balancer.Stop()
	require.Empty(t, errs)

	// check that iptables was cleaned up
	for c, _ := range mipt.chains {
		require.Contains(t, builtinChains, c)
	}
}
