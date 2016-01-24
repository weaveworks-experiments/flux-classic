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

func NewClient(c etcd.Client) Client {
	return Client{
		Client:  c,
		KeysAPI: etcd.NewKeysAPI(c),
	}
}

func NewClientFromEnv() (Client, error) {
	addr := os.Getenv("ETCD_ADDRESS")
	if addr == "" {
		return Client{}, fmt.Errorf("ETCD_ADDRESS environment variable not set; expected the address of the etcd server")
	}

	c, err := etcd.New(etcd.Config{Endpoints: []string{addr}})
	if err != nil {
		return Client{}, err
	}

	return NewClient(c), nil
}

func (c *Client) EtcdClient() etcd.Client {
	return c.Client
}
