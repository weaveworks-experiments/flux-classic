package store

import (
	"time"
)

type Cluster interface {
	// EnsureConfig checks that the supplied cluster-wide
	// configuration is either not present (in which case it is set),
	// or if present, is equal to that supplied.
	EnsureConfig(config interface{}) error
	Heartbeat(ttl time.Duration) error
	EndSession() error
}
