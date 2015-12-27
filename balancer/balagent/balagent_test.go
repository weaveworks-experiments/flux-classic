package balagent

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/squaremo/ambergreen/common/daemon"
	"github.com/squaremo/ambergreen/common/store/inmem"
)

func TestBalancerAgent(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	a := &BalancerAgent{
		errorSink: daemon.NewErrorSink(),
		store:     inmem.NewInMemStore(),
		filename: fmt.Sprintf("%s/balagent-%d", os.TempDir(),
			rng.Intn(1000000)),
	}

	a.start()

	select {
	case err := <-a.errorSink:
		t.Fatal(err)
	default:
	}

	a.Stop()
}
