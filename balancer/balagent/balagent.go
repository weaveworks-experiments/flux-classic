package balagent

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"

	"github.com/weaveworks/flux/balancer/etcdcontrol"
	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
)

type BalancerAgent struct {
	errorSink  daemon.ErrorSink
	store      store.Store
	filename   string
	template   *template.Template
	reloadCmd  string
	updates    <-chan model.ServiceUpdate
	controller daemon.Component
	stop       chan struct{}
	stopped    bool

	services chan Services

	// for tests:
	generated        chan struct{}
	updaterStopped   chan struct{}
	generatorStopped chan struct{}
}

type Services map[string]*model.Service

// For use in templates
func (Services) Getenv(name string) string {
	return os.Getenv(name)
}

func StartBalancerAgent(args []string, errorSink daemon.ErrorSink) *BalancerAgent {
	a := &BalancerAgent{errorSink: errorSink}

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
	fs.StringVar(&a.reloadCmd, "c", "",
		"command to run each time the file is regenerated")

	var err error
	if err = fs.Parse(args[1:]); err != nil {
		return err
	}

	a.template, err = template.ParseFiles(templateFile)
	if err != nil {
		return fmt.Errorf(`unable to parse file "%s": %s`, templateFile, err)
	}

	a.store, err = etcdstore.NewFromEnv()
	return err
}

func (a *BalancerAgent) start() error {
	updates := make(chan model.ServiceUpdate)
	a.updates = updates
	a.controller = daemon.Restart(time.Second*10, etcdcontrol.NewListener(a.store, updates))(a.errorSink)

	a.stop = make(chan struct{})
	a.services = make(chan Services, 1)
	go a.updater()
	go a.generator()
	return nil
}

func (a *BalancerAgent) Stop() {
	if a.controller != nil {
		a.controller.Stop()
		a.controller = nil
	}

	if !a.stopped {
		close(a.stop)
		a.stopped = true
	}
}

// Aggregates service updates, and sends snapshots of the full state
// to the generator goroutine.
func (a *BalancerAgent) updater() {
	services := make(Services)

	for {
		select {
		case <-a.stop:
			if a.updaterStopped != nil {
				a.updaterStopped <- struct{}{}
			}
			return

		case u := <-a.updates:
			if u.Reset {
				services = make(Services)
			}

			for name, service := range u.Updates {
				if service == nil {
					delete(services, name)
				} else {
					services[name] = service
				}
			}
		}

		// Copy services to send to the generator
		s := make(Services)
		for k, v := range services {
			s[k] = v
		}

		// remove any pending item sitting in a.services:
		select {
		case <-a.services:
		default:
		}

		a.services <- s
	}
}

func (a *BalancerAgent) generator() {
	for {
		select {
		case <-a.stop:
			if a.generatorStopped != nil {
				a.generatorStopped <- struct{}{}
			}
			return

		case services := <-a.services:
			if err := a.regenerate(services); err != nil {
				a.errorSink.Post(err)
			}

			if a.generated != nil {
				a.generated <- struct{}{}
			}
		}
	}
}

func (a *BalancerAgent) regenerate(services Services) error {
	f, err := ioutil.TempFile(path.Dir(a.filename), path.Base(a.filename))
	if err != nil {
		return err
	}

	tmpname := f.Name()
	defer func() {
		if f != nil {
			f.Close()
			os.Remove(tmpname)
		}
	}()

	if err := a.template.Execute(f, services); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpname, a.filename); err != nil {
		return err
	}

	f = nil

	return a.runReloadCmd()
}

func (a *BalancerAgent) runReloadCmd() error {
	if a.reloadCmd == "" {
		return nil
	}

	done := make(chan error)
	go func() {
		cmd := exec.Command("sh", "-c", a.reloadCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			done <- err
			return
		}

		done <- cmd.Wait()
	}()

	timeout := time.NewTimer(10 * time.Second)
	select {
	case <-timeout.C:
		return fmt.Errorf("timeout waiting for reload command to complete")

	case err := <-done:
		return err
	}
}
