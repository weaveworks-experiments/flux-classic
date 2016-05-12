package main

import (
	"os/exec"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/balancer"
	"github.com/weaveworks/flux/balancer/serverside"
	"github.com/weaveworks/flux/common/daemon"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	// the server-side balancer is wired to the agent to receive
	// local instance information

	instanceUpdates := make(chan agent.InstanceUpdate)
	instanceUpdatesReset := make(chan struct{}, 1)

	daemon.Main(&agent.AgentConfig{
		InstanceUpdates:      instanceUpdates,
		InstanceUpdatesReset: instanceUpdatesReset,
	}, &balancer.BalancerConfig{
		IPTablesCmd: iptables,
	}, &serverside.Config{
		InstanceUpdates:      instanceUpdates,
		InstanceUpdatesReset: instanceUpdatesReset,
	})
}
