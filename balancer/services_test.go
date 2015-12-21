package balancer

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/ambergreen/balancer/events"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/common/daemon"
)

func TestServices(t *testing.T) {
	nc := netConfig{
		chain:  "AMBERGREEN",
		bridge: "lo",
	}

	mipt := newMockIPTables(t)
	ipTables := newIPTables(nc, mipt.cmd)
	ipTables.start()

	errorSink := daemon.NewErrorSink()
	updates := make(chan model.ServiceUpdate)
	done := make(chan struct{}, 1)
	svcs := servicesConfig{
		netConfig:    nc,
		updates:      updates,
		ipTables:     ipTables,
		eventHandler: events.DiscardOthers{},
		errorSink:    errorSink,
		done:         done,
	}.new()

	// Add a service
	svc := model.Service{
		Name:     "service",
		Protocol: "tcp",
		IP:       net.ParseIP("127.42.0.1"),
		Port:     8888,
		Instances: []model.Instance{
			{
				Name:  "foo",
				Group: "bar",
				IP:    net.ParseIP("127.0.0.1"),
				Port:  10000,
			},
		},
	}
	updates <- model.ServiceUpdate{Service: svc}
	<-done

	require.Len(t, mipt.chains["nat AMBERGREEN"], 1)
	require.Len(t, mipt.chains["filter AMBERGREEN"], 0)
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat AMBERGREEN"][0], " "))

	// Update it
	svc.Instances = []model.Instance{
		{
			Name:  "foo",
			Group: "bar",
			IP:    net.ParseIP("127.0.0.1"),
			Port:  10001,
		},
	}
	updates <- model.ServiceUpdate{Service: svc}
	<-done

	require.Len(t, mipt.chains["nat AMBERGREEN"], 1)
	require.Len(t, mipt.chains["filter AMBERGREEN"], 0)
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat AMBERGREEN"][0], " "))

	// Update with no instances
	svc.Instances = nil
	updates <- model.ServiceUpdate{Service: svc}
	<-done

	require.Len(t, mipt.chains["nat AMBERGREEN"], 0)
	require.Len(t, mipt.chains["filter AMBERGREEN"], 1)
	require.Equal(t, "-p tcp -d 127.42.0.1 --dport 8888 -j REJECT",
		strings.Join(mipt.chains["filter AMBERGREEN"][0], " "))

	// And once more for luck:
	updates <- model.ServiceUpdate{Service: svc}
	<-done

	require.Len(t, mipt.chains["nat AMBERGREEN"], 0)
	require.Len(t, mipt.chains["filter AMBERGREEN"], 1)
	require.Equal(t, "-p tcp -d 127.42.0.1 --dport 8888 -j REJECT",
		strings.Join(mipt.chains["filter AMBERGREEN"][0], " "))

	// Delete it
	updates <- model.ServiceUpdate{Service: svc, Delete: true}
	<-done

	require.Len(t, mipt.chains["nat AMBERGREEN"], 0)
	require.Len(t, mipt.chains["filter AMBERGREEN"], 0)

	// Delete it, even though it doesn't exist
	updates <- model.ServiceUpdate{Service: svc, Delete: true}
	<-done

	svcs.close()
}
