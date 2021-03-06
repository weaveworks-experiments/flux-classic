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
	"time"

	"github.com/stretchr/testify/require"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/netutil"
	"github.com/weaveworks/flux/common/store"
	"github.com/weaveworks/flux/common/store/inmem"
)

type ingressInstance struct {
	Address netutil.IPPort
	store.IngressInstance
}

type ingressInstances []ingressInstance

func (iis ingressInstances) Len() int { return len(iis) }

func (iis ingressInstances) Less(i, j int) bool {
	return iis[i].Address.LessThan(iis[j].Address)
}

func (iis ingressInstances) Swap(i, j int) {
	t := iis[i]
	iis[i] = iis[j]
	iis[j] = t
}

func sortIngressInstances(iis map[netutil.IPPort]store.IngressInstance) ingressInstances {
	var a ingressInstances
	for addr, ii := range iis {
		a = append(a,
			ingressInstance{Address: addr, IngressInstance: ii})
	}

	sort.Sort(a)
	return a
}

func newBalancerAgentConfig(t *testing.T) *BalancerAgentConfig {
	dir, err := ioutil.TempDir("", "balagent_test")
	require.Nil(t, err)

	return &BalancerAgentConfig{
		store:             inmem.NewInMem().Store("test session"),
		filename:          path.Join(dir, "output"),
		reconnectInterval: 100 * time.Millisecond,
		generated:         make(chan struct{}),
	}
}

func (cf *BalancerAgentConfig) start(t *testing.T) (daemon.Component, daemon.ErrorSink) {
	start, err := cf.Prepare()
	require.Nil(t, err)
	errs := daemon.NewErrorSink()
	return start(errs), errs
}

func cleanup(cf *BalancerAgentConfig, t *testing.T) {
	require.Nil(t, os.RemoveAll(path.Dir(cf.filename)))
}

func TestBalancerAgent(t *testing.T) {
	cf := newBalancerAgentConfig(t)
	defer cleanup(cf, t)

	tmpl := template.New("template")
	tmpl.Funcs(template.FuncMap{"sortIngressInstances": sortIngressInstances})

	var err error
	cf.template, err = tmpl.Parse(`
{{$HOME := .Getenv "HOME"}}
{{if len $HOME}}{{else}}No $HOME{{end}}
{{range $svcname, $svc := .}}{{$svcname}}:{{range sortIngressInstances $svc.IngressInstances}} ({{.Address}}, {{.Weight}}){{end}}
{{end}}`)
	require.Nil(t, err)

	// Add an initial service with no instances:
	require.Nil(t, cf.store.AddService("service1", store.Service{
		Protocol: "http",
		Address:  netutil.ParseIPPortPtr("1.2.3.4:80"),
	}))

	comp, errs := cf.start(t)
	<-cf.generated
	requireFile(t, cf.filename, "service1:")

	// Add an instance to the service:
	require.Nil(t, cf.store.AddIngressInstance("service1",
		*netutil.ParseIPPortPtr("5.6.7.8:1"),
		store.IngressInstance{Weight: 42}))
	<-cf.generated
	requireFile(t, cf.filename, "service1: (5.6.7.8:1, 42)")

	// And another instance:
	require.Nil(t, cf.store.AddIngressInstance("service1",
		*netutil.ParseIPPortPtr("9.10.11.12:2"),
		store.IngressInstance{Weight: 7}))
	<-cf.generated
	requireFile(t, cf.filename, "service1: (5.6.7.8:1, 42) (9.10.11.12:2, 7)")

	// Add another service:
	require.Nil(t, cf.store.AddService("service2", store.Service{
		Protocol: "http",
	}))
	<-cf.generated
	requireFile(t, cf.filename, `service1: (5.6.7.8:1, 42) (9.10.11.12:2, 7)
service2:`)

	// Delete first service:
	require.Nil(t, cf.store.RemoveService("service1"))
	<-cf.generated
	requireFile(t, cf.filename, "service2:")

	comp.Stop()
	require.Len(t, errs, 0)

	// Check that all temporary files got deleted
	require.Nil(t, os.Remove(cf.filename))
	fis, err := ioutil.ReadDir(path.Dir(cf.filename))
	require.Nil(t, err)
	require.Empty(t, fis)
}

func requireFile(t *testing.T, filename string, expect string) {
	data, err := ioutil.ReadFile(filename)
	require.Nil(t, err)
	require.Equal(t, expect, strings.TrimSpace(string(data)))
}

func TestBadTemplate(t *testing.T) {
	cf := newBalancerAgentConfig(t)
	defer cleanup(cf, t)

	var err error
	cf.template, err = template.New("template").Parse("{{.service1.wut}}")
	require.Nil(t, err)

	// Add an initial service with no instances:
	require.Nil(t, cf.store.AddService("service1", store.Service{
		Protocol: "http",
		Address:  netutil.ParseIPPortPtr("1.2.3.4:80"),
	}))

	comp, errs := cf.start(t)
	<-cf.generated
	comp.Stop()
	require.Len(t, errs, 1)
}

func TestReloadCmd(t *testing.T) {
	cf := newBalancerAgentConfig(t)
	defer cleanup(cf, t)

	var err error
	cf.template, err = template.New("template").Parse("ok")
	require.Nil(t, err)

	require.Nil(t, cf.store.AddService("service1", store.Service{
		Protocol: "http",
		Address:  netutil.ParseIPPortPtr("1.2.3.4:90"),
	}))

	tmp := cf.filename + "-copy"
	cf.reloadCmd = fmt.Sprintf("cp %s %s", cf.filename, tmp)

	comp, errs := cf.start(t)
	<-cf.generated
	requireFile(t, tmp, "ok")

	comp.Stop()
	require.Len(t, errs, 0)
}
