package main

import (
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer/balagent"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/version"
)

func main() {
	log.Infof("flux balagent version %s", version.Version())
	daemon.Main(func(errs daemon.ErrorSink) daemon.Component {
		return balagent.StartBalancerAgent(os.Args, errs)
	})
}
