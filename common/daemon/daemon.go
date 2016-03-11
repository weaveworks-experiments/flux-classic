package daemon

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func Main(starters ...StartFunc) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	errorSink := NewErrorSink()

	components := make([]Component, len(starters))
	for i, start := range starters {
		components[i] = start(errorSink)
	}

	exitCode := 0
	var exitSignal os.Signal

	select {
	case err := <-errorSink:
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
	case exitSignal = <-sigs:
	}

	for i := len(components) - 1; i >= 0; i-- {
		d := components[i]
		if d != nil {
			d.Stop()
		}
	}

	if sig, ok := exitSignal.(syscall.Signal); ok {
		// Now we have cleaned up, re-kill the process with
		// the signal in order to produce a signal exit
		// status:
		signal.Reset(sig)
		syscall.Kill(syscall.Getpid(), sig)
	} else if exitSignal != nil {
		fmt.Fprintln(os.Stderr, "Exiting with signal ", sig)
	}

	os.Exit(exitCode)
}
