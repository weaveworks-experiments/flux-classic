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

	if err := a.start(nil); err != nil {
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

	if fs.NArg() != 1 {
		return fmt.Errorf("expected <service> argument")
	}
	a.service = fs.Arg(0)

	a.template, err = template.ParseFiles(templateFile)
	if err != nil {
		return fmt.Errorf(`unable to parse file "%s": %s`, templateFile, err)
	}

	return nil
}

func (a *BalancerAgent) start(tick chan<- struct{}) error {
	controller, err := etcdcontrol.NewListener(a.store, a.errorSink)
	if err != nil {
		return err
	}

	a.controller = controller
	a.stop = make(chan struct{})
	go a.run(tick)
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

func maybeWait(tick chan<- struct{}) {
	if tick != nil {
		tick <- struct{}{}
	}
}

func (a *BalancerAgent) run(tick chan<- struct{}) {
	updates := a.controller.Updates()

	for {
		select {
		case <-a.stop:
			maybeWait(tick)
			break

		case u := <-updates:
			if err := a.handleUpdate(&u); err != nil {
				a.errorSink.Post(err)
				maybeWait(tick)
				break
			}
		}
		maybeWait(tick)
	}
}

func (a *BalancerAgent) handleUpdate(u *model.ServiceUpdate) error {
	if u.Name == a.service {
		if err := a.runTemplate(u); err != nil {
			return err
		}
	}
	return nil
}

func (a *BalancerAgent) runTemplate(u *model.ServiceUpdate) error {
	output := new(bytes.Buffer)
	var (
		outfile *os.File
		err     error
	)
	if err = a.template.Execute(output, u); err != nil {
		return err
	}
	if outfile, err = os.Create(a.filename); err != nil {
		return err
	}
	if _, err = outfile.Write(output.Bytes()); err != nil {
		return err
	}
	return nil
}
