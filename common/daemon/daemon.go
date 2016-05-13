package daemon

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

func run(start StartFunc) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	errorSink := NewErrorSink()

	d := start(errorSink)
	exitCode := 0
	var exitSignal os.Signal

again:
	select {
	case err := <-errorSink:
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			exitCode = 1
		}
	case exitSignal = <-sigs:
		if exitSignal == syscall.SIGQUIT {
			var buf [8192]byte
			length := runtime.Stack(buf[:], true)
			fmt.Fprint(os.Stderr, string(buf[:length]))
			goto again
		}
	}

	if d != nil {
		d.Stop()
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
