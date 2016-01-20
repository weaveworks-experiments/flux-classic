package daemon

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

type Daemon interface {
	Stop()
}

func Main(start func(args []string, errorSink ErrorSink) Daemon) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	errorSink := NewErrorSink()

	d := start(os.Args, errorSink)
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

	d.Stop()

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
