package balancer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/balancer/fatal"
)

func TestDaemon(t *testing.T) {
	ipTables := newMockIPTables(t)
	fatalSink := fatal.New()
	i := Start([]string{"balancer"}, fatalSink, ipTables.cmd)

	select {
	case err := <-fatalSink:
		t.Fatal(err)
	default:
	}

	i.Stop()

	// check that iptables was cleaned up
	for c, _ := range ipTables.chains {
		require.Contains(t, builtinChains, c)
	}
}
