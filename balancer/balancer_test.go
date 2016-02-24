package balancer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/eventlogger"
	"github.com/weaveworks/flux/common/daemon"
)

func TestBalancer(t *testing.T) {
	ipTables := newMockIPTables(t)
	d := BalancerDaemon{
		errorSink:    daemon.NewErrorSink(),
		ipTablesCmd:  ipTables.cmd,
		eventHandler: eventlogger.EventLogger{},
		netConfig: netConfig{
			chain:  "FLUX",
			bridge: "docker0",
		},
	}

	d.Start()
	require.Empty(t, d.errorSink)
	d.Stop()

	// check that iptables was cleaned up
	for c, _ := range ipTables.chains {
		require.Contains(t, builtinChains, c)
	}
}
