package balagent

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"text/template"

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
	template   *template.Template
	service    string
	controller model.Controller
	stop       chan struct{}
	tick       chan struct{}

	services Services
}

type Services map[string]*model.Service

func (Services) Getenv(name string) string {
	return os.Getenv(name)
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

	fs.StringVar(&a.filename, "o", "/tmp/services",
		"name of file to generate")
	var templateFile string
	fs.StringVar(&templateFile, "i", "nginx.tmpl",
		"name of template file with which to generate the output file")

	var err error
	if err = fs.Parse(args[1:]); err != nil {
		return err
	}

	a.template, err = template.ParseFiles(templateFile)
	if err != nil {
		return fmt.Errorf(`unable to parse file "%s": %s`, templateFile, err)
	}

	a.store = etcdstore.NewFromEnv()
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
			if a.tick != nil {
				a.tick <- struct{}{}
			}
			return

		case u := <-updates:
			a.handleUpdate(&u)
			if a.tick != nil {
				a.tick <- struct{}{}
			}
		}
	}
}

func (a *BalancerAgent) handleUpdate(u *model.ServiceUpdate) {
	if u.Delete {
		delete(a.services, u.Name)
	} else {
		a.services[u.Name] = &u.Service
	}

	if err := a.runTemplate(); err != nil {
		a.errorSink.Post(err)
	}
}

func (a *BalancerAgent) runTemplate() error {
	output := new(bytes.Buffer)
	err := a.template.Execute(output, a.services)
	if err != nil {
		return err
	}

	outfile, err := os.Create(a.filename)
	if err != nil {
		return err
	}

	if _, err = outfile.Write(output.Bytes()); err != nil {
		return err
	}

	if err = outfile.Close(); err != nil {
		return err
	}

	return nil
}
