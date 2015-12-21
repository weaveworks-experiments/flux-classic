package balancer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/common/daemon"
)

func TestDaemon(t *testing.T) {
	ipTables := newMockIPTables(t)
	errorSink := daemon.NewErrorSink()
	i := Start([]string{"balancer"}, errorSink, ipTables.cmd)

	select {
	case err := <-errorSink:
		t.Fatal(err)
	default:
	}

	i.Stop()

	// check that iptables was cleaned up
	for c, _ := range ipTables.chains {
		require.Contains(t, builtinChains, c)
	}
}
