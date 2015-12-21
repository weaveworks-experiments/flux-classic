package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/squaremo/ambergreen/balancer"
	"github.com/squaremo/ambergreen/common/daemon"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	// Catch some signals for which we want to clean up on exit
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	errorSink := daemon.New()
	bal := balancer.Start(os.Args, errorSink, iptables)

	exitCode := 0
	var exitSignal os.Signal

	select {
	case err := <-errorSink:
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	case exitSignal = <-sigs:
		exitCode = 2
	}

	bal.Stop()

	if sig, ok := exitSignal.(syscall.Signal); ok {
		// Now we have cleaned up, re-kill the process with
		// the signal in order to produce a signal exit
		// status:
		signal.Reset(sig)
		syscall.Kill(syscall.Getpid(), sig)
	}

	os.Exit(exitCode)
}
