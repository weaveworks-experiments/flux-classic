package heartbeat

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store/inmem"
)

func TestHeartbeat(t *testing.T) {
	st := inmem.NewInMemStore()
	conf := HeartbeatConfig{
		st,
		1 * time.Second,
		"host foo",
		&data.Host{IPAddress: "192.168.3.34"},
	}
	sink := daemon.NewErrorSink()
	hb := conf.Start(sink)
	hosts, err := st.GetHosts()
	require.Nil(t, err)
	require.Len(t, hosts, 1)
	hb.Stop()
}
