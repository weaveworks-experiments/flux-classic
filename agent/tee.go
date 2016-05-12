package agent

import (
	"errors"
	"reflect"

	"github.com/weaveworks/flux/common/daemon"
)

func Tee(in interface{}, outs ...interface{}) daemon.StartFunc {
	inT := reflect.TypeOf(in)
	if inT.Kind() != reflect.Chan || (inT.ChanDir()&reflect.RecvDir) == 0 {
		panic("Non-channel (or non-receive channel) passed to Tee")
	}

	for _, out := range outs {
		outT := reflect.TypeOf(out)
		if outT.Kind() != reflect.Chan || (outT.ChanDir()&reflect.SendDir) == 0 {
			panic("Non-channel (or non-send channel) passed to Tee")
		}

		if outT.Elem() != inT.Elem() {
			panic("Tee output channel type does not match input channel type")
		}
	}

	return daemon.SimpleComponent(func(stop <-chan struct{}, errs daemon.ErrorSink) {
		inCases := []reflect.SelectCase{
			reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(in),
			},
			reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(stop),
			},
		}

		for {
			chosen, recv, ok := reflect.Select(inCases)
			if chosen == 1 {
				// receive on stop channel
				return
			}

			if !ok {
				errs.Post(errors.New("Tee input channel closed"))
			}

			for _, out := range outs {
				reflect.ValueOf(out).Send(recv)
			}
		}
	})
}
