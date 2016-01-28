package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer/balagent"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/version"
)

func main() {
	log.Infof("flux balagent version %s", version.Version())
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		return balagent.StartBalancerAgent(args, errs)
	})
}
