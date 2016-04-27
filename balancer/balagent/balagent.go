package balagent

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"text/template"
	"time"

	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/etcdstore"
)

type templateFileSlot struct {
	slot **template.Template
}

func templateFileDependency(slot **template.Template) daemon.DependencySlot {
	return templateFileSlot{slot}
}

func (s templateFileSlot) Key() daemon.DependencyKey {
	return s
}

func (s templateFileSlot) Assign(value interface{}) {
	*s.slot = value.(*template.Template)
}

type templateFileConfig struct {
	templateFile string
}

func (templateFileSlot) MakeConfig() daemon.DependencyConfig {
	return &templateFileConfig{}
}

func (cf *templateFileConfig) Populate(deps *daemon.Dependencies) {
	deps.StringVar(&cf.templateFile, "i", "nginx.tmpl",
		"name of template file with which to generate the output file")
}

func (cf *templateFileConfig) MakeValue() (interface{}, error) {
	tmpl, err := template.ParseFiles(cf.templateFile)
	if err != nil {
		return nil, fmt.Errorf(`unable to parse file "%s": %s`, cf.templateFile, err)
	}

	return tmpl, nil
}

type BalancerAgentConfig struct {
	filename          string
	reloadCmd         string
	template          *template.Template
	store             store.Store
	reconnectInterval time.Duration

	// for tests:
	generated chan struct{}
}

func (cf *BalancerAgentConfig) Populate(deps *daemon.Dependencies) {
	deps.StringVar(&cf.filename, "o", "/tmp/services",
		"name of file to generate")
	deps.StringVar(&cf.reloadCmd, "c", "",
		"command to run each time the file is regenerated")
	deps.Dependency(templateFileDependency(&cf.template))
	deps.Dependency(etcdstore.StoreDependency(&cf.store))
}

func (cf *BalancerAgentConfig) Prepare() (daemon.StartFunc, error) {
	if cf.reconnectInterval == 0 {
		cf.reconnectInterval = 10 * time.Second
	}

	updates := make(chan model.ServiceUpdate)
	services := make(chan Services, 1)

	return daemon.Aggregate(
		daemon.Restart(cf.reconnectInterval,
			model.WatchServicesStartFunc(cf.store, false, updates)),
		daemon.SimpleComponent(updater{updates, services}.run),
		daemon.SimpleComponent(generator{cf, services}.run)), nil
}

type Services map[string]*model.Service

// For use in templates
func (Services) Getenv(name string) string {
	return os.Getenv(name)
}

// Aggregates service updates, and sends snapshots of the full state
// to the generator goroutine.
type updater struct {
	updates  <-chan model.ServiceUpdate
	services chan Services
}

func (u updater) run(stop <-chan struct{}, _ daemon.ErrorSink) {
	services := make(Services)

	for {
		select {
		case <-stop:
			return

		case updates := <-u.updates:
			if updates.Reset {
				services = make(Services)
			}

			for name, service := range updates.Updates {
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

		// remove any pending item sitting in servicesCh:
		select {
		case <-u.services:
		default:
		}

		u.services <- s
	}
}

type generator struct {
	*BalancerAgentConfig
	services <-chan Services
}

func (g generator) run(stop <-chan struct{}, errs daemon.ErrorSink) {
	for {
		select {
		case <-stop:
			return

		case services := <-g.services:
			errs.Post(g.regenerate(services))
			if g.generated != nil {
				g.generated <- struct{}{}
			}
		}
	}
}

func (g generator) regenerate(services Services) error {
	f, err := ioutil.TempFile(path.Dir(g.filename), path.Base(g.filename))
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

	if err := g.template.Execute(f, services); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Rename(tmpname, g.filename); err != nil {
		return err
	}

	f = nil
	return g.runReloadCmd()
}

func (g generator) runReloadCmd() error {
	if g.reloadCmd == "" {
		return nil
	}

	done := make(chan error)
	go func() {
		cmd := exec.Command("sh", "-c", g.reloadCmd)
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
