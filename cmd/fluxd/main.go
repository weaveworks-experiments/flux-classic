package main

import (
	"os/exec"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/balancer"
	"github.com/weaveworks/flux/common/daemon"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	daemon.Main(&agent.AgentConfig{},
		&balancer.BalancerConfig{IPTablesCmd: iptables})
}
