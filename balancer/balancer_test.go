package balancer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/flux/common/daemon"
)

func TestBalancer(t *testing.T) {
	ipTables := newMockIPTables(t)
	errorSink := daemon.NewErrorSink()
	b := StartBalancer([]string{"balancer"}, errorSink, ipTables.cmd)

	select {
	case err := <-errorSink:
		t.Fatal(err)
	default:
	}

	b.Stop()

	// check that iptables was cleaned up
	for c, _ := range ipTables.chains {
		require.Contains(t, builtinChains, c)
	}
}
