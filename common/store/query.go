package store

import (
	"github.com/squaremo/ambergreen/common/data"
)

func SelectInstances(store Store, sel data.Selector, fun InstanceFunc) {
	store.ForeachServiceInstance(nil, func(_ string, i string, d data.Instance) {
		if sel.Includes(d) {
			fun(i, d)
		}
	})
}

func SelectServiceInstances(store Store, service string, s data.Selector, fi InstanceFunc) {
	store.ForeachInstance(service, func(i string, d data.Instance) {
		if s.Includes(d) {
			fi(i, d)
		}
	})
}
