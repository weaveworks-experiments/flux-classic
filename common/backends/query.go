package backends

import (
	"github.com/squaremo/ambergreen/common/data"
)

func SelectInstances(be *Backend, sel data.Selector, fun InstanceFunc) {
	be.ForeachServiceInstance(nil, func(_ string, i string, d data.Instance) {
		if sel.Includes(d) {
			fun(i, d)
		}
	})
}

func SelectServiceInstances(be *Backend, service string, s data.Selector, fi InstanceFunc) {
	be.ForeachInstance(service, func(i string, d data.Instance) {
		if s.Includes(d) {
			fi(i, d)
		}
	})
}
