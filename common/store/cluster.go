package store

import (
	"time"
)

type Cluster interface {
	Heartbeat(ttl time.Duration) error
	EndSession() error
}
