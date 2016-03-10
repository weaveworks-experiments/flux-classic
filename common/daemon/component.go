package daemon

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

// A StartFunc starts a component
type StartFunc func(ErrorSink) Component

// A component is simply something that can be stopped
type Component interface {
	// Stop the component.  The implementation of this component
	// should be synchronous: When it returns, all resources of
	// the component should have been released, and any activity
	// associated with the component has ceased.
	Stop()
}

// Supervise a component, restarting it upon errors
func Restart(interval time.Duration, start StartFunc) StartFunc {
	f := func(errs ErrorSink, stop <-chan struct{}, finished chan<- struct{}) {
		defer close(finished)
		stopped := false
		for {
			errs := NewErrorSink()
			tStart := time.Now()
			d := start(errs)
			select {
			case <-stop:
				stopped = true
			case err := <-errs:
				log.WithError(err).Error()
			}

			if d != nil {
				t := time.AfterFunc(10*time.Second, func() {
					errs.Post(fmt.Errorf("timed out stopping component"))
				})
				d.Stop()
				t.Stop()
			}

			if stopped {
				return
			}

			wait := tStart.Add(interval).Sub(time.Now())
			if wait > 0 {
				t := time.NewTimer(wait)
				select {
				case <-stop:
					return
				case <-t.C:
				}

				t.Stop()
			}
		}
	}

	return func(errs ErrorSink) Component {
		stop := make(chan struct{})
		finished := make(chan struct{})
		go f(errs, stop, finished)
		return &stopper{stop: stop, finished: finished}
	}
}

func SimpleComponent(f func(stop <-chan struct{}, errs ErrorSink)) StartFunc {
	return func(errs ErrorSink) Component {
		stop := make(chan struct{})
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			f(stop, errs)
		}()
		return &stopper{stop: stop, finished: finished}
	}
}

type stopper struct {
	stop     chan<- struct{}
	finished <-chan struct{}
	stopped  sync.Once
}

func (stopper *stopper) Stop() {
	stopper.stopped.Do(func() {
		close(stopper.stop)
	})
	<-stopper.finished
}

// Because Component only has a single method, it's convenient to have
// a way to dress up a stop function as a Component.  This also takes
// care of ensuring that the stop function is only called once.
func StopFunc(f func()) Component {
	return &stopFunc{f: f}
}

type stopFunc struct {
	f       func()
	stopped sync.Once
}

func (sf *stopFunc) Stop() {
	sf.stopped.Do(sf.f)
}

func Aggregate(startFuncs ...StartFunc) StartFunc {
	return func(errs ErrorSink) Component {
		stopFuncs := make([]func(), len(startFuncs))
		for i := range startFuncs {
			stopFuncs[i] = startFuncs[i](errs).Stop
		}

		return StopFunc(func() { Par(stopFuncs...) })
	}
}

func Par(funcs ...func()) {
	done := make(chan struct{}, len(funcs))
	for _, f := range funcs {
		go func(f func()) {
			defer func() { done <- struct{}{} }()
			f()
		}(f)
	}

	for range funcs {
		<-done
	}
}
