package main

import (
	"os/exec"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/balancer"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/version"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	log.Infof("fluxd version %s", version.Version())
	daemon.ConfigsMain(&agent.AgentConfig{},
		&balancer.BalancerConfig{IPTablesCmd: iptables})
}
