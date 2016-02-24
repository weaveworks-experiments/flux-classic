package daemon

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

// A component is simply something that can be stopped
type Component interface {
	Stop()
}

// A StartFunc starts a component
type StartFunc func(ErrorSink) Component

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

type stopper struct {
	stop     chan<- struct{}
	finished <-chan struct{}
	lock     sync.Mutex
}

func (stopper *stopper) Stop() {
	stopper.lock.Lock()

	if stopper.stop != nil {
		defer stopper.lock.Unlock()
		close(stopper.stop)
		stopper.stop = nil
	} else {
		stopper.lock.Unlock()
	}

	<-stopper.finished
}
