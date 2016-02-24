package etcdutil

import (
	"fmt"
	"os"

	etcd "github.com/coreos/etcd/client"
)

type Client struct {
	Client etcd.Client
	etcd.KeysAPI
}

func NewClient(endpoints ...string) (Client, error) {
	c, err := etcd.New(etcd.Config{Endpoints: endpoints})
	if err != nil {
		return Client{}, err
	}

	return Client{
		Client:  c,
		KeysAPI: etcd.NewKeysAPI(c),
	}, nil
}

func NewClientFromEnv() (Client, error) {
	addr := os.Getenv("ETCD_ADDRESS")
	if addr == "" {
		return Client{}, fmt.Errorf("ETCD_ADDRESS environment variable not set; expected the address of the etcd server")
	}

	return NewClient(addr)
}

func (c *Client) EtcdClient() etcd.Client {
	return c.Client
}
