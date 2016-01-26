package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/squaremo/flux/balancer/balagent"
	"github.com/squaremo/flux/common/daemon"
	"github.com/squaremo/flux/common/version"
)

func main() {
	log.Infof("flux balagent version %s", version.Version())
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		return balagent.StartBalancerAgent(args, errs)
	})
}
