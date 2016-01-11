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
		return balancer.StartBalancer(args, errs, iptables)
	})
}
