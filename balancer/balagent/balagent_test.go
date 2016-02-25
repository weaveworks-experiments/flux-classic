package balagent

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/balancer/model"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/data"
	"github.com/weaveworks/flux/common/store/inmem"
)

type instances []model.Instance

func (insts instances) Len() int { return len(insts) }

func (insts instances) Less(i, j int) bool {
	return insts[i].Name < insts[j].Name
}

func (insts instances) Swap(i, j int) {
	t := insts[i]
	insts[i] = insts[j]
	insts[j] = t
}

func sortInsts(a interface{}) interface{} {
	insts := instances(a.([]model.Instance))
	sort.Sort(insts)
	return insts
}

func newBalancerAgent(t *testing.T) *BalancerAgent {
	dir, err := ioutil.TempDir("", "balagent_test")
	require.Nil(t, err)

	return &BalancerAgent{
		errorSink: daemon.NewErrorSink(),
		store:     inmem.NewInMemStore(),
		filename:  path.Join(dir, "output"),

		generated:        make(chan struct{}),
		updaterStopped:   make(chan struct{}),
		generatorStopped: make(chan struct{}),
	}
}

func cleanup(a *BalancerAgent, t *testing.T) {
	require.Nil(t, os.RemoveAll(path.Dir(a.filename)))
}

func TestBalancerAgent(t *testing.T) {
	a := newBalancerAgent(t)
	defer cleanup(a, t)

	tmpl := template.New("template")
	tmpl.Funcs(template.FuncMap{"sortInsts": sortInsts})

	var err error
	a.template, err = tmpl.Parse(`
{{$HOME := .Getenv "HOME"}}
{{if len $HOME}}{{else}}No $HOME{{end}}
{{range .}}{{.Name}}:{{range sortInsts .Instances}} ({{.Name}}, {{.IP}}:{{.Port}}){{end}}
{{end}}`)
	require.Nil(t, err)

	// Add an initial service with no instances:
	require.Nil(t, a.store.AddService("service1", data.Service{
		Protocol: "http",
		Address:  "1.2.3.4",
	}))

	a.start()
	<-a.generated
	requireFile(t, a.filename, "service1:")

	// Add an instance to the service:
	require.Nil(t, a.store.AddInstance("service1", "inst1",
		data.Instance{State: data.LIVE, Address: "5.6.7.8", Port: 1}))
	<-a.generated
	requireFile(t, a.filename, "service1: (inst1, 5.6.7.8:1)")

	// And another instance:
	require.Nil(t, a.store.AddInstance("service1", "inst2",
		data.Instance{State: data.LIVE, Address: "9.10.11.12", Port: 2}))
	<-a.generated
	requireFile(t, a.filename, "service1: (inst1, 5.6.7.8:1) (inst2, 9.10.11.12:2)")

	// Add another service:
	require.Nil(t, a.store.AddService("service2", data.Service{
		Protocol: "http",
		Address:  "13.14.15.16",
	}))
	<-a.generated
	requireFile(t, a.filename, `service1: (inst1, 5.6.7.8:1) (inst2, 9.10.11.12:2)
service2:`)

	// Delete first service:
	require.Nil(t, a.store.RemoveService("service1"))
	<-a.generated
	requireFile(t, a.filename, "service2:")

	a.Stop()
	<-a.updaterStopped
	<-a.generatorStopped
	require.Len(t, a.errorSink, 0)

	// Check that all temporary files got deleted
	require.Nil(t, os.Remove(a.filename))
	fis, err := ioutil.ReadDir(path.Dir(a.filename))
	require.Nil(t, err)
	require.Empty(t, fis)
}

func requireFile(t *testing.T, filename string, expect string) {
	data, err := ioutil.ReadFile(filename)
	require.Nil(t, err)
	require.Equal(t, expect, strings.TrimSpace(string(data)))
}

func TestBadTemplate(t *testing.T) {
	a := newBalancerAgent(t)
	defer cleanup(a, t)

	var err error
	a.template, err = template.New("template").Parse("{{.service1.wut}}")
	require.Nil(t, err)

	// Add an initial service with no instances:
	require.Nil(t, a.store.AddService("service1", data.Service{
		Protocol: "http",
		Address:  "1.2.3.4",
	}))

	a.start()
	<-a.generated
	a.Stop()
	<-a.updaterStopped
	<-a.generatorStopped
	require.Len(t, a.errorSink, 1)
}

func TestReloadCmd(t *testing.T) {
	a := newBalancerAgent(t)
	defer cleanup(a, t)

	var err error
	a.template, err = template.New("template").Parse("ok")
	require.Nil(t, err)

	require.Nil(t, a.store.AddService("service1", data.Service{
		Protocol: "http",
		Address:  "1.2.3.4",
	}))

	tmp := a.filename + "-copy"
	a.reloadCmd = fmt.Sprintf("cp %s %s", a.filename, tmp)

	a.start()
	<-a.generated
	requireFile(t, tmp, "ok")

	a.Stop()
	<-a.updaterStopped
	<-a.generatorStopped
}
