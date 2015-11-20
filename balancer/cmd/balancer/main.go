package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/squaremo/ambergreen/balancer"
	"github.com/squaremo/ambergreen/balancer/fatal"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	exitCode := 0
	defer os.Exit(exitCode)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	fatalSink := fatal.New()
	i := balancer.Start(os.Args, fatalSink, iptables)
	defer i.Stop()

	select {
	case <-sigs:
	case err := <-fatalSink:
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}
