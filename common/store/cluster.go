package store

import (
	"golang.org/x/net/context"
	"time"

	"github.com/weaveworks/flux/common/daemon"
)

type Cluster interface {
	GetHosts() ([]*Host, error)
	Heartbeat(identity string, ttl time.Duration, state *Host) error
	DeregisterHost(identity string) error
	WatchHosts(ctx context.Context, changes chan<- HostChange, errs daemon.ErrorSink)
}
