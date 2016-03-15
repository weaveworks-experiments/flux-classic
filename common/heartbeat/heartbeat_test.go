package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store/inmem"
)

// Test that starting a heartbeat puts a record in the store, and that
// the heartbeat will maintain the record.
func TestHeartbeat(t *testing.T) {
	st := inmem.NewInMemStore()
	ttl := time.Duration(100 * time.Millisecond)
	hostID := "host foo"

	conf := HeartbeatConfig{
		st,
		ttl,
		hostID,
		&data.Host{IPAddress: "192.168.3.34"},
	}
	sink := daemon.NewErrorSink()

	// starting the heartbeat puts a record in straight away
	hb := conf.Start(sink)
	hosts, err := st.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 1)

	// thereafter, we get the record maintained
	time.Sleep(2 * ttl)
	updateCount, err := st.GetHeartbeat(hostID)
	require.Nil(t, err)
	require.True(t, updateCount > 0)

	// heartbeats stop after the heartbeater is stopped
	hb.Stop()
	updateCount1, err := st.GetHeartbeat(hostID)
	require.Nil(t, err)
	time.Sleep(2 * ttl)
	updateCount2, err := st.GetHeartbeat(hostID)
	require.Nil(t, err)
	require.Equal(t, updateCount1, updateCount2)
	// NB I don't check that the host record has been removed, since
	// that's a property of the store rather than the heartbeater.
}
