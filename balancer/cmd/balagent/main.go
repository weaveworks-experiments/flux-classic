package main

import (
	"github.com/squaremo/ambergreen/balancer/balagent"
	"github.com/squaremo/ambergreen/common/daemon"
)

func main() {
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		return balagent.StartBalancerAgent(args, errs)
	})
}
