package backends

import (
	"github.com/squaremo/ambergreen/common/data"
)

type ServiceFunc func(string, data.Service)
type InstanceFunc func(string, data.Instance)
type ServiceInstanceFunc func(string, string, data.Instance)
