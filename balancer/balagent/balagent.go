package balagent

import (
	"flag"
	"fmt"

	"github.com/squaremo/ambergreen/balancer/etcdcontrol"
	"github.com/squaremo/ambergreen/balancer/model"
	"github.com/squaremo/ambergreen/common/daemon"
	"github.com/squaremo/ambergreen/common/store"
	"github.com/squaremo/ambergreen/common/store/etcdstore"
)

type BalancerAgent struct {
	errorSink  daemon.ErrorSink
	store      store.Store
	filename   string
	controller model.Controller
	stop       chan struct{}

	services map[string]*model.Service
}

func StartBalancerAgent(args []string, errorSink daemon.ErrorSink) *BalancerAgent {
	a := &BalancerAgent{
		errorSink: errorSink,
		store:     etcdstore.NewFromEnv(),
	}

	if err := a.parseArgs(args); err != nil {
		errorSink.Post(err)
		return a
	}

	if err := a.start(); err != nil {
		errorSink.Post(err)
	}

	return a
}

func (a *BalancerAgent) parseArgs(args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)

	fs.StringVar(&a.filename, "f", "/tmp/services",
		"name of file to generate")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("excess command line arguments")
	}

	return nil
}

func (a *BalancerAgent) start() error {
	controller, err := etcdcontrol.NewListener(a.store, a.errorSink)
	if err != nil {
		return err
	}

	a.controller = controller
	a.stop = make(chan struct{})
	go a.run()
	return nil
}

func (a *BalancerAgent) Stop() {
	if a.controller != nil {
		a.controller.Close()
	}

	if a.stop != nil {
		close(a.stop)
		a.stop = nil
	}
}

func (a *BalancerAgent) run() {
	a.services = make(map[string]*model.Service)
	updates := a.controller.Updates()

	for {
		select {
		case <-a.stop:
			return

		case u := <-updates:
			a.handleUpdate(&u)
		}
	}
}

func (a *BalancerAgent) handleUpdate(u *model.ServiceUpdate) {
	if u.Delete {
		delete(a.services, u.Name)
	} else {
		a.services[u.Name] = &u.Service
	}
}
