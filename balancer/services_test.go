package balancer

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/squaremo/flux/balancer/events"
	"github.com/squaremo/flux/balancer/model"
	"github.com/squaremo/flux/common/daemon"
)

func requireForwarding(t *testing.T, mipt *mockIPTables) {
	require.Len(t, mipt.chains["nat FLUX"], 1)
	require.Len(t, mipt.chains["filter FLUX"], 0)
	// NB regexp related to service IP and port given in test case
	require.Regexp(t, "^-p tcp -d 127\\.42\\.0\\.1 --dport 8888 -j DNAT --to-destination 127\\.0\\.0\\.1:\\d+$", strings.Join(mipt.chains["nat FLUX"][0], " "))
}

func requireRejecting(t *testing.T, mipt *mockIPTables) {
	require.Len(t, mipt.chains["nat FLUX"], 0)
	require.Len(t, mipt.chains["filter FLUX"], 1)
	require.Equal(t, "-p tcp -d 127.42.0.1 --dport 8888 -j REJECT",
		strings.Join(mipt.chains["filter FLUX"][0], " "))
}

func requireNotForwarding(t *testing.T, mipt *mockIPTables) {
	require.Len(t, mipt.chains["nat FLUX"], 0)
	require.Len(t, mipt.chains["filter FLUX"], 0)
}

func TestServices(t *testing.T) {
	nc := netConfig{
		chain:  "FLUX",
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

	ip := net.ParseIP("127.42.0.1")
	port := 8888

	// Add a service
	svc := model.Service{
		Name:     "service",
		Protocol: "tcp",
		IP:       ip,
		Port:     port,
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
	requireForwarding(t, &mipt)

	insts := []model.Instance{
		{
			Name:  "foo",
			Group: "bar",
			IP:    net.ParseIP("127.0.0.1"),
			Port:  10001,
		},
	}

	// Update it
	svc.Instances = insts
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireForwarding(t, &mipt)

	// forwarding -> rejecting
	svc.Instances = nil
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireRejecting(t, &mipt)

	// rejecting -> not forwarding
	svc.IP = nil
	svc.Port = 0
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireNotForwarding(t, &mipt)

	// not forwarding -> forwarding
	svc.IP = ip
	svc.Port = port
	svc.Instances = insts
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireForwarding(t, &mipt)

	// Now back the other way
	// forwarding -> not forwarding
	svc.IP = nil
	svc.Port = 0
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireNotForwarding(t, &mipt)

	// not forwarding -> rejecting
	svc.IP = ip
	svc.Port = port
	svc.Instances = nil
	updates <- model.ServiceUpdate{Service: svc}
	<-done
	requireRejecting(t, &mipt)

	// Delete it
	updates <- model.ServiceUpdate{Service: svc, Delete: true}
	<-done
	requireNotForwarding(t, &mipt)

	// Delete it, even though it doesn't exist
	updates <- model.ServiceUpdate{Service: svc, Delete: true}
	<-done

	svcs.close()
}
