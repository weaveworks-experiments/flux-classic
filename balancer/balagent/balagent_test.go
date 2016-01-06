package balagent

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"text/template"
	"time"

	"github.com/squaremo/ambergreen/common/daemon"
	"github.com/squaremo/ambergreen/common/data"
	"github.com/squaremo/ambergreen/common/store/inmem"

	"github.com/stretchr/testify/require"
)

func fileContentsEqual(t *testing.T, filename, contents string) bool {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	return contents == string(bytes)
}

func requireFileContents(t *testing.T, filename, contents string) {
	require.True(t, fileContentsEqual(t, filename, contents))
}

func requireNoError(t *testing.T, a *BalancerAgent) {
	select {
	case err := <-a.errorSink:
		t.Fatal(err)
	default:
	}
}

func requireError(t *testing.T, a *BalancerAgent) {
	select {
	case <-a.errorSink:
		return
	default:
		t.Fatal(fmt.Errorf("expected error, but executed cleanly"))
	}
}

type testcase func(*BalancerAgent, <-chan struct{})

func testCase(t *testing.T, tc testcase) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	a := &BalancerAgent{
		errorSink: daemon.NewErrorSink(),
		store:     inmem.NewInMemStore(),
		filename: fmt.Sprintf("%s/balagent-%d", os.TempDir(),
			rng.Intn(1000000)),
	}

	tick := make(chan struct{})
	a.start(tick)

	tc(a, tick)
	a.Stop()
}

func trivialFixture(a *BalancerAgent, tmpl string) {
	a.template = template.Must(template.New("test").Parse(tmpl))
	a.service = "test-svc"
	a.store.AddService("test-svc", data.Service{
		Address:  "10.5.7.34",
		Port:     8000,
		Protocol: "http",
	})
	a.store.AddInstance("test-svc", "test-instance", data.Instance{
		Address: "192.168.66.77",
		Port:    7000,
	})
}

func TestTrivialSuccess(t *testing.T) {
	testCase(t, func(a *BalancerAgent, tick <-chan struct{}) {
		trivialFixture(a, "{{.Name}}")
		<-tick
		requireFileContents(t, a.filename, "test-svc")
		requireNoError(t, a)
	})
}

func TestBadTemplate(t *testing.T) {
	testCase(t, func(a *BalancerAgent, tick <-chan struct{}) {
		trivialFixture(a, "{{.wut}}")
		<-tick
		requireError(t, a)
	})
}

func TestInstanceUpdate(t *testing.T) {
	testCase(t, func(a *BalancerAgent, tick <-chan struct{}) {
		trivialFixture(a, "{{range .Service.Instances}}|{{.Name}}{{end}}")
		<-tick
		requireFileContents(t, a.filename, "|test-instance")
		a.store.AddInstance("test-svc", "test-instance2", data.Instance{
			Address: "192.168.99.100",
			Port:    9000,
		})
		<-tick
		require.True(t, fileContentsEqual(t, a.filename, "|test-instance|test-instance2") ||
			fileContentsEqual(t, a.filename, "|test-instance2|test-instance"))
		requireNoError(t, a)
	})
}
