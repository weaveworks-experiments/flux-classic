package daemon

import (
	"time"
)

// A StartFunc constructor that runs a procedure periodically.

func Ticker(interval time.Duration, tick func(errs ErrorSink)) StartFunc {
	return SimpleComponent(func(stop <-chan struct{}, errs ErrorSink) {
		tick(errs)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				tick(errs)

			case <-stop:
				return
			}
		}
	})
}
