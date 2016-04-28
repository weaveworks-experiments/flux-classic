package daemon

import (
	"time"
)

/*

A StartFunc constructor that runs a procedure periodically.

*/
func Ticker(interval time.Duration, tick func(time.Time) error) StartFunc {
	return func(errorSink ErrorSink) Component {
		c := &tickerComponent{
			tickFunc: tick,
		}
		go c.run(interval, errorSink)
		return c
	}
}

// ----------

type tickerComponent struct {
	cancel   chan struct{}
	ticker   *time.Ticker
	tickFunc func(time.Time) error
}

func (component *tickerComponent) Stop() {
	if component.ticker != nil {
		component.ticker.Stop()
	}
	if component.cancel != nil {
		component.cancel <- struct{}{}
	}
}

func (component *tickerComponent) run(interval time.Duration, errs ErrorSink) {
	component.cancel = make(chan struct{})
	component.ticker = time.NewTicker(interval)
	for {
		select {
		case t := <-component.ticker.C:
			if err := component.tickFunc(t); err != nil {
				errs.Post(err)
				return
			}
		case <-component.cancel:
			return
		}
	}
}
