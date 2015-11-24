package balancer

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/fatal"
	"github.com/squaremo/ambergreen/balancer/model"
)

func TestServices(t *testing.T) {
	nc := netConfig{
		chain:  "AMBERGRIS",
		bridge: "lo",
	}

	mipt := newMockIPTables(t)
	ipTables := newIPTables(nc, mipt.cmd)
	ipTables.start()

	fatalSink := fatal.New()
	updates := make(chan model.ServiceUpdate)
	done := make(chan struct{}, 1)
	svcs := servicesConfig{
		netConfig:    nc,
		updates:      updates,
		ipTables:     ipTables,
		eventHandler: events.DiscardOthers{},
		fatalSink:    fatalSink,
		done:         done,
	}.new()

	// Add a service
	key := model.MakeServiceKey("tcp", net.ParseIP("127.42.0.1"), 8888)
	updates <- model.ServiceUpdate{
		ServiceKey: key,
		ServiceInfo: &model.ServiceInfo{
			Instances: []model.Instance{
				model.MakeInstance("foo", "bar",
					net.ParseIP("127.0.0.1"), 10000),
			},
		},
	}
	<-done

	require.Len(t, mipt.chains["nat AMBERGRIS"], 1)
	require.Len(t, mipt.chains["filter AMBERGRIS"], 0)
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat AMBERGRIS"][0], " "))

	// Update it
	updates <- model.ServiceUpdate{
		ServiceKey: key,
		ServiceInfo: &model.ServiceInfo{
			Instances: []model.Instance{
				model.MakeInstance("foo", "bar",
					net.ParseIP("127.0.0.1"), 10001),
			},
		},
	}
	<-done

	require.Len(t, mipt.chains["nat AMBERGRIS"], 1)
	require.Len(t, mipt.chains["filter AMBERGRIS"], 0)
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat AMBERGRIS"][0], " "))

	// Update with no instances
	updates <- model.ServiceUpdate{
		ServiceKey:  key,
		ServiceInfo: &model.ServiceInfo{},
	}
	<-done

	require.Len(t, mipt.chains["nat AMBERGRIS"], 0)
	require.Len(t, mipt.chains["filter AMBERGRIS"], 1)
	require.Equal(t, "-p tcp -d 127.42.0.1 --dport 8888 -j REJECT",
		strings.Join(mipt.chains["filter AMBERGRIS"][0], " "))

	// And once more for luck:
	updates <- model.ServiceUpdate{
		ServiceKey:  key,
		ServiceInfo: &model.ServiceInfo{},
	}
	<-done

	require.Len(t, mipt.chains["nat AMBERGRIS"], 0)
	require.Len(t, mipt.chains["filter AMBERGRIS"], 1)
	require.Equal(t, "-p tcp -d 127.42.0.1 --dport 8888 -j REJECT",
		strings.Join(mipt.chains["filter AMBERGRIS"][0], " "))

	// Delete it
	updates <- model.ServiceUpdate{
		ServiceKey: key,
	}
	<-done

	require.Len(t, mipt.chains["nat AMBERGRIS"], 0)
	require.Len(t, mipt.chains["filter AMBERGRIS"], 0)

	// Delete it, even though it doesn't exist
	updates <- model.ServiceUpdate{
		ServiceKey: key,
	}
	<-done

	svcs.close()
}
