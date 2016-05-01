package balancer

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/events"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
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
	done := make(chan model.ServiceUpdate, 1)
	svcs := servicesConfig{
		netConfig:    nc,
		updates:      updates,
		ipTables:     ipTables,
		eventHandler: events.NullHandler{},
		errorSink:    errorSink,
		done:         done,
	}.start()

	ip := net.ParseIP("127.42.0.1")
	port := 8888

	update := func(svc model.Service, reset bool) {
		updates <- model.ServiceUpdate{
			Updates: map[string]*model.Service{svc.Name: &svc},
			Reset:   reset,
		}
		<-done
	}

	// Add a service
	svc := model.Service{
		Name:     "service",
		Protocol: "tcp",
		Address:  &netutil.IPPort{ip, port},
		Instances: map[string]netutil.IPPort{
			"foo": netutil.IPPort{net.ParseIP("127.0.0.1"), 10000},
		},
	}
	update(svc, true)
	requireForwarding(t, &mipt)

	insts := map[string]netutil.IPPort{
		"foo": netutil.IPPort{net.ParseIP("127.0.0.1"), 10001},
	}

	// Update it
	svc.Instances = insts
	update(svc, false)
	requireForwarding(t, &mipt)

	// forwarding -> rejecting
	svc.Instances = nil
	update(svc, false)
	requireRejecting(t, &mipt)

	// rejecting -> forwarding
	svc.Instances = insts
	update(svc, false)
	requireForwarding(t, &mipt)

	// Delete it
	updates <- model.ServiceUpdate{
		Updates: map[string]*model.Service{svc.Name: nil},
	}
	<-done
	requireNotForwarding(t, &mipt)

	// Delete it, even though it doesn't exist
	updates <- model.ServiceUpdate{
		Updates: map[string]*model.Service{svc.Name: nil},
	}
	<-done

	svcs.stop()
}
