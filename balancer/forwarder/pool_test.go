package forwarder

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/netutil"
)

func TestPoolOfOne(t *testing.T) {
	// Empty pool
	pool := NewInstancePool()
	require.Nil(t, pool.PickInstance())

	pool.UpdateInstances(nil)
	require.Nil(t, pool.PickInstance())

	// Add an instance
	addr := netutil.IPPort{net.IP{192, 168, 3, 135}, 32768}
	pool.UpdateInstances(map[string]netutil.IPPort{"foo": addr})
	picked := pool.PickInstance()
	require.Equal(t, addr, picked.Address)

	pool.Succeeded(picked)
	picked = pool.PickInstance()
	require.Equal(t, addr, picked.Address)

	// Even if the instance is failed, it's the only one in the
	// pool, so it should still get picked
	pool.Failed(picked)
	require.Empty(t, pool.ready)
	require.Equal(t, addr, pool.PickInstance().Address)

	// Remove instance
	pool.UpdateInstances(nil)
	require.Nil(t, pool.PickInstance())

	pool.Stop()
}

type timer struct {
	time.Time
	next time.Time
}

func (t *timer) Reset(d time.Duration) bool {
	t.next = t.Time.Add(d)
	return true
}

func (t *timer) Stop() bool {
	return true
}

func (t *timer) now() time.Time {
	return t.Time
}

func TestFailAndRetryInstance(t *testing.T) {
	pool := NewInstancePool()
	tm := timer{Time: time.Now()}
	pool.timer = &tm
	pool.now = tm.now

	inst1 := netutil.IPPort{net.IP{192, 168, 3, 101}, 1001}
	inst2 := netutil.IPPort{net.IP{192, 168, 3, 102}, 1002}
	inst3 := netutil.IPPort{net.IP{192, 168, 3, 103}, 1003}

	pool.UpdateInstances(map[string]netutil.IPPort{"inst1": inst1})
	picked1 := pool.PickInstance()
	require.Equal(t, inst1, picked1.Address)
	pool.Failed(picked1)
	require.Equal(t, tm.Add(retry_interval_base), tm.next)

	// incidentally test that failed instances remain failed, when
	// included in an update
	pool.UpdateInstances(map[string]netutil.IPPort{
		"inst1": inst1,
		"inst2": inst2,
	})

	// check that inst2 (ready) is preferred to inst1 (failed)
	for i := 0; i < 20; i++ {
		picked2 := pool.PickInstance()
		require.Equal(t, inst2, picked2.Address)
		pool.Succeeded(picked2)
	}

	// Fail inst2 and retry inst1
	tm.Time = tm.next
	pool.Failed(pool.PickInstance())
	pool.processRetries(tm.Time)

	// Now inst1 should get picked
	picked1 = pool.PickInstance()
	require.Equal(t, inst1, picked1.Address)

	// Add a ready inst3
	pool.UpdateInstances(map[string]netutil.IPPort{
		"inst1": inst1,
		"inst2": inst2,
		"inst3": inst3,
	})

	// check that inst3 (ready) is preferred to inst1 (retrying)
	// and inst2 (failed)
	for i := 0; i < 20; i++ {
		picked3 := pool.PickInstance()
		require.Equal(t, inst3, picked3.Address)
		pool.Succeeded(picked3)
	}

	pool.Succeeded(picked1)
	pool.UpdateInstances(map[string]netutil.IPPort{
		"inst1": inst1,
		"inst2": inst2,
	})

	// inst3 has gone, inst2 is failed, so inst1 is preferred
	for i := 0; i < 20; i++ {
		picked1 = pool.PickInstance()
		require.Equal(t, inst1, picked1.Address)
		pool.Succeeded(picked1)
	}

	pool.Stop()
}

func TestRetryBackoff(t *testing.T) {
	pool := NewInstancePool()
	tm := timer{Time: time.Now()}
	pool.timer = &tm
	pool.now = tm.now

	addr := netutil.IPPort{net.IP{192, 168, 3, 135}, 32768}
	pool.UpdateInstances(map[string]netutil.IPPort{"foo": addr})

	for i := uint(0); i < 5; i++ {
		// invariant: the instance is ready here
		pool.Failed(pool.PickInstance())
		require.Empty(t, pool.ready)
		require.Equal(t, tm.Add((1<<i)*retry_interval_base), tm.next)
		tm.Time = tm.next
		pool.processRetries(tm.Time)
		require.NotEmpty(t, pool.ready)
	}

	pool.Stop()
}
