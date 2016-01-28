package prometheus

import (
	"time"

	log "github.com/Sirupsen/logrus"
	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/etcdutil"
	"github.com/weaveworks/flux/common/netutil"
)

const TTL = 5 * time.Minute

type advertiser struct {
	address    string
	etcdClient etcdutil.Client
	errorSink  daemon.ErrorSink

	stopCh chan struct{}
}

func newAdvertiser(cf Config) (*advertiser, error) {
	address, err := netutil.NormalizeHostPort(cf.AdvertiseAddr,
		"tcp", false)
	if err != nil {
		return nil, err
	}

	return &advertiser{
		address:    address,
		etcdClient: cf.EtcdClient,
		errorSink:  cf.ErrorSink,
	}, nil
}

func (a *advertiser) start() {
	a.stopCh = make(chan struct{})
	go a.run(a.stopCh)
}

func (a *advertiser) stop() {
	if a.stopCh != nil {
		close(a.stopCh)
		a.stopCh = nil
	}
}

func (a *advertiser) run(stop <-chan struct{}) {
	ctx := context.Background()

	resp, err := a.etcdClient.CreateInOrder(ctx,
		"/weave-flux/prometheus-targets", a.address,
		&etcd.CreateInOrderOptions{TTL: TTL})
	if err != nil {
		a.errorSink.Post(err)
		return
	}

	key := resp.Node.Key
	t := time.NewTicker(TTL / 2)
	defer t.Stop()

	for {
		select {
		case <-t.C:
		case <-stop:
			return
		}

		// Don't treat errors as fatal here; maybe etcd will
		// come back
		_, err := a.etcdClient.Set(ctx, key, a.address,
			&etcd.SetOptions{TTL: TTL})
		if err != nil {
			log.Error(err)
		}
	}
}
