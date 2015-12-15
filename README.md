## Service controller for containers

Amber lets you define _services_ which are load-balanced over a set of
Docker containers. The containers are automatically enrolled in
services, according to selection rules you supply.

### How to run it

Assuming you have [the prerequisites](#prerequisites), you need to run
the agent and the balancer on each host. `./bin/run-amber` will run
them as Docker containers.

You need to provide an etcd endpoint, and an IP address for the
host. The IP address is used as the address for mapped ports (i.e.,
when you use `-p or -P` when running the container).

Say I'm running everything (including etcd) on one host, with the IP
address `192.168.99.100`:

```bash
HOST_IP=192.168.99.100
ETCD_ADDRESS=http://$HOST_IP:4001
export ETCD_ADDRESS HOST_IP
./bin/run-amber
```

### How to use it

Interaction with the system is via a command-line tool,
`amberctl`. This can be used as a binary, or with the script
`bin/amberctl` which invokes the binary as a Docker image. Both need
`ETCD_ADDRESS` in the environment.

To define a service and select containers to enrol, you use

```
amberctl service <service> <IP address> <port> [--protocol=tcp|http]
amberctl select <service> <name> <address spec> [<selector>...]
```

The `<service>` name, IP address and port are arbitrary, and supplied
by you. They should not correspond to an address already in use. It's
this IP address and port that clients should connect to when using the
service, so you may also want to arrange for it to be in DNS, or
`/etc/hosts` for client containers. The protocol option controls
whether client connections should be treated as HTTP, or plain TCP;
using HTTP means better statistics can be collected (but not all
servies will use HTTP).

The selection `<name>` is simply a handle so you can undo the
selection later. The `<address spec>` tells Amber how to connect to an
enrolled instance. It is either

 * `--mapped <port>`, which means use the host's IP address, along with the host port that is mapped to the given container port; or,

 * `--fixed <port>` which means use the IP address reported by `docker
   inspect -f '{{.NetworkSettings.IPAddress}}`, along with the given
   port.

`--mapped` is for when you are mapping ports on the host using `-p` or
`-P` when running containers. `--fixed` is for when your containers
have a network connecting them (e.g., if you are using a Weave
network) and don't need to map ports.

The selectors are a set of rules for matching containers. Some simple
rules are `--image` and `--tag`, which match the image name and tag
respectively (the tag is the bit after the colon in the image, which
is often a version number). For example, a service definition could be

```bash
amberctl service search-svc 10.128.0.1 80
amberctl select search-svc default --image searchapi
```

Any container using the image `searchapi` will be enrolled as an
instance of `search-svc`, and the service will be available on each
host at 10.128.0.1:80.

See the [command-line README](amberctl/README.md#readme) for details
on defining services, selecting and deselecting containers, and
querying the system.

### Prerequisites

Amber assumes you have an etcd installation handy. If you don't, it's
easy to run one under Docker for the purpose of kicking the
tires. Assuming you have a host with Docker running and accessible on
`HOST_IP`, do

```bash
docker run -d -p 4001:4001 \
       -e "ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:4001"
       -e "ETCD_ADVERTISE_CLIENT_URLS=http://$HOST_IP:4001"
       quay.io/coreos/etcd
```

Now you have an etcd available on `http://$HOST_IP:4001`.

If you run the web interface, you will also need an instance of
Prometheus. See the [web interface README](web/README.md) for more
details.

### Disclaimer

Amber is a work in progress. There are rough edges, and areas where
expedience has driven the design.
