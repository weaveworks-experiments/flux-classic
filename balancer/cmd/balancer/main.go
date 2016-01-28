package main

import (
	"os/exec"

	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/balancer"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/version"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	log.Infof("flux balancer version %s", version.Version())
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		b, err := balancer.NewBalancer(args, errs, iptables)
		if err != nil {
			errs.Post(err)
			return nil
		}

		b.Start()
		return b
	})
}
