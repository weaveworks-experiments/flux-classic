package main

import (
	"github.com/squaremo/flux/balancer/balagent"
	"github.com/squaremo/flux/common/daemon"
)

func main() {
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		return balagent.StartBalancerAgent(args, errs)
	})
}
