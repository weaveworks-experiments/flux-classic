package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store/inmem"
)

// Test that starting a heartbeat puts a record in the store, and that
// the heartbeat will maintain the record.
func TestHeartbeat(t *testing.T) {
	sessionID := "test sesh"
	back := inmem.NewInMem()
	st := back.Store(sessionID)
	ttl := time.Duration(100 * time.Millisecond)

	conf := &HeartbeatConfig{st, ttl, nil}
	sink := daemon.NewErrorSink()

	// starting the heartbeat puts a record in straight away
	hb := conf.StartFunc()(sink)

	// thereafter, we get the record maintained
	time.Sleep(2 * ttl)
	updateCount, err := back.GetHeartbeat(sessionID)
	require.Nil(t, err)
	require.True(t, updateCount > 0)

	// heartbeats stop after the heartbeater is stopped
	hb.Stop()
	updateCount1, err := back.GetHeartbeat(sessionID)
	require.Nil(t, err)
	time.Sleep(2 * ttl)
	updateCount2, err := back.GetHeartbeat(sessionID)
	require.Nil(t, err)
	require.Equal(t, updateCount1, updateCount2)
}
