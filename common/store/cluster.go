package store

import (
	"golang.org/x/net/context"
	"time"

	"github.com/weaveworks/flux/common/data"
)

type Cluster interface {
	GetHosts() ([]*data.Host, error)
	Heartbeat(identity string, ttl time.Duration, state *data.Host) error
	DeregisterHost(identity string) error
	WatchHosts(ctx context.Context, changes chan<- data.HostChange)
}
