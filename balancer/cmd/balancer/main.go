package main

import (
	"os/exec"

	"github.com/squaremo/flux/balancer"
	"github.com/squaremo/flux/common/daemon"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
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
