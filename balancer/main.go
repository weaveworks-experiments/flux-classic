package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/squaremo/ambergreen/balancer/interceptor"
)

func iptables(args []string) ([]byte, error) {
	return exec.Command("iptables", args...).CombinedOutput()
}

func main() {
	exitCode := 0
	defer os.Exit(exitCode)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	i := interceptor.Start(os.Args, iptables)
	defer i.Stop()

	select {
	case <-sigs:
	case err := <-i.Fatal:
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}
