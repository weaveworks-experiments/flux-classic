package main

import (
	"os/exec"

	"github.com/squaremo/ambergreen/balancer"
	"github.com/squaremo/ambergreen/common/daemon"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	daemon.Main(func(args []string, errs daemon.ErrorSink) daemon.Daemon {
		return balancer.StartBalancer(args, errs, iptables)
	})
}
