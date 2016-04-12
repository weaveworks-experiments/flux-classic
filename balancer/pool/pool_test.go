package pool

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/model"
)

func TestPoolOfOne(t *testing.T) {
	// if there's one instance, it always gets picked
	pool := NewInstancePool()
	require.Nil(t, pool.PickInstance())

	pool.UpdateInstances([]model.Instance{})
	require.Nil(t, pool.PickInstance())

	instance := model.Instance{
		Name: "foo instance",
		IP:   net.IP{192, 168, 3, 135},
		Port: 32768,
	}

	pool.UpdateInstances([]model.Instance{instance})
	picked := pool.PickInstance()
	require.NotNil(t, picked)
	require.Equal(t, &instance, picked.Instance())
	require.Equal(t, &instance, pool.PickInstance().Instance())
	picked.Fail()
	require.Nil(t, pool.PickActiveInstance())
	require.Equal(t, &instance, pool.PickInstance().Instance())

	pool.UpdateInstances([]model.Instance{})
	require.Nil(t, pool.PickInstance())
}

func TestFailAndRetryInstance(t *testing.T) {
	// if you fail an instance, it doesn't get picked until you retry it
	pool := NewInstancePool()
	require.Nil(t, pool.PickInstance())

	instances := []model.Instance{
		model.Instance{
			Name: "instance one",
			IP:   net.IP{192, 168, 3, 101},
			Port: 32768,
		},
		model.Instance{
			Name: "instance two",
			IP:   net.IP{192, 168, 3, 135},
			Port: 32761,
		},
	}

	pool.UpdateInstances(instances[:1])
	picked := pool.PickInstance()
	failedInstance := picked.Instance()
	picked.Fail()

	// incidentally test that failed instances remain failed, when
	// included in an update
	pool.UpdateInstances(instances)
	okInstance := &instances[1]

	// OK sure, this is an approximation to "doesn't get picked"
	for i := 0; i < 100; i++ {
		require.Equal(t, okInstance, pool.PickInstance().Instance())
	}

	picked = pool.PickInstance() // = okInstance
	// add failedInstance back
	pool.ReactivateRetries(time.Now().Add(time.Duration(retry_initial_interval) * time.Second))
	picked.Fail()
	// now should be only failed instance in the active instances
	okInstance, failedInstance = failedInstance, okInstance // make these names accurate again

	for i := 0; i < 100; i++ {
		require.Equal(t, okInstance, pool.PickInstance().Instance())
	}
}

func TestRetryBackoff(t *testing.T) {
	pool := NewInstancePool()
	require.Nil(t, pool.PickInstance())

	instance := model.Instance{
		Name: "instance one",
		IP:   net.IP{192, 168, 3, 101},
		Port: 32768,
	}

	pool.UpdateInstances([]model.Instance{instance})
	require.NotNil(t, pool.PickInstance())

	now := time.Now()

	for retry := retry_initial_interval; retry < retry_abandon_threshold; retry *= retry_backoff_factor {
		// invariant: the instance is active here
		pool.PickActiveInstance().Fail()
		pool.ReactivateRetries(now.Add(1 * time.Second))
		require.Nil(t, pool.PickActiveInstance())
		pool.ReactivateRetries(now.Add(time.Duration(retry+1) * time.Second))
		require.NotNil(t, pool.PickActiveInstance())
	}
	// it should be abandoned this time
	pool.PickActiveInstance().Fail()
	pool.ReactivateRetries(now.Add(time.Duration(retry_abandon_threshold+1) * time.Second))
	require.Nil(t, pool.PickInstance())
}

func TestRetryKeep(t *testing.T) {
	pool := NewInstancePool()
	require.Nil(t, pool.PickInstance())

	instance := model.Instance{
		Name: "janky instance",
		IP:   net.IP{192, 168, 35, 10},
		Port: 32716,
	}

	pool.UpdateInstances([]model.Instance{instance})
	require.NotNil(t, pool.PickInstance())

	now := time.Now()

	retry := retry_initial_interval

	pool.PickActiveInstance().Fail()
	pool.ReactivateRetries(now.Add(time.Duration(retry+1) * time.Second))
	pool.PickActiveInstance().Fail()

	// having been reactivated once, reactivating again after the same
	// interval shouldn't pick it up
	pool.ReactivateRetries(now.Add(time.Duration(retry+1) * time.Second))
	require.Nil(t, pool.PickActiveInstance())

	pool.PickInstance().Keep() // should reset the retry interval and return to active service
	pool.ReactivateRetries(now.Add(time.Duration(retry+1) * time.Second))
	require.NotNil(t, pool.PickActiveInstance())
}
