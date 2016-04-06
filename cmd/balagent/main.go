package main

import (
	"github.com/weaveworks/flux/balancer/balagent"
	"github.com/weaveworks/flux/common/daemon"
)

func main() {
	daemon.Main(&balagent.BalancerAgentConfig{})
}
