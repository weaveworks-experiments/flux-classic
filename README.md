## Service controller for containers

Weave Flux lets you define _services_ which are accessed on the same
IP address and port everywhere, and load-balanced over a set of Docker
containers. The containers are automatically enrolled in services
according to selection rules you supply.

### How to run it

Assuming you have [the prerequisites](#prerequisites), you need to run
the agent and the balancer on each host. `./bin/run-flux` will run
them both, as Docker containers.

You need to provide the agent and the balancer with an etcd endpoint
in an environment entry `ETCD_ADDRESS`; and, the agent with an IP
address for the host, as `HOST_IP`. The IP address is used as the
address for mapped ports (i.e., if you use `-p` or `-P` when running
the container).

Say I'm running everything (including etcd) on one host, with the IP
address `192.168.99.100`:

```bash
HOST_IP=192.168.99.100
ETCD_ADDRESS=http://$HOST_IP:2379
export ETCD_ADDRESS HOST_IP
./bin/run-flux
```

### How to use it

Interaction with the system is via a command-line tool,
`fluxctl`. This can be used as a binary, or with the script
`bin/fluxctl` which invokes the binary as a Docker image. Both need
`ETCD_ADDRESS` in the environment.

To define a service, you use

```
fluxctl service <service> --address <IP address>:<port> [ --protocol http ]
```

The IP address and port are chosen by you. The IP address is a virtual
IP address; it shouldn't correspond to a device, or an address already
in use. It's best to pick an address range for your services that
won't be used anywhere else, and give each service an IP address from
that range.

It's this IP address and port that clients will connect to when using
the service, so you may also want to arrange for it to be in DNS, or
`/etc/hosts` for client containers.

The `--protocol` option controls whether client connections should be
treated as HTTP, or plain TCP; using HTTP means HTTP-specific metrics
can be collected, but not all services will use HTTP.

To enrol containers in the service, use

```
fluxctl select <service> <rule> <address spec> [<selector>...]
```

The selection `<rule>` name is simply a handle so you can undo the
selection later.

The `<address spec>` tells Weave Flux how to reach a container. There
are two alternatives: using mapped ports, or assuming a common
network. The corresponding options are:

 * `--port-mapped <port>`, which means use the host's IP address,
   along with the host port that is mapped to the given container
   port. This is for when you are mapping ports on the host using `-p`
   or `-P` with `docker run ...`.

 * `--port-fixed <port>` which means use the IP address reported by
   Docker (i.e., as from `docker inspect ...`), along with the given
   port. This is for when your containers have a network connecting
   them (e.g., if you are using a Weave network) and don't need to map
   ports.

The selectors are a set of rules for matching containers. Some simple
rules are `--image` and `--tag`, which match the image name and tag
respectively (the tag is the bit after the colon in the image, which
is often a version number).

For example, a service definition could be

```bash
fluxctl service search-svc --address 10.128.0.1:80 --protocol http
fluxctl select search-svc default --port-mapped 8080 --image searchapi
```

Any container using the image `searchapi` will be enrolled as an
instance of `search-svc`, and the service will be available on each
host at 10.128.0.1:80.

See the
[command-line reference](blob/master/site/fluxctl/index.md#readme) for
details on defining services, selecting containers, and querying the
system.

### Running the web interface

Weave Flux has a web interface that shows the statistics gathered from
the services.

The web interface needs to know how to connect to etcd (using the
environment entry `ETCD_ADDRESS`) and to Prometheus (using the
environment entry `PROMETHEUS_ADDRESS`). Some help with running
Prometheus, and configuring the system to use it, is given in the web
interface [README](blob/master/web/README.md#readme).

To run it under Docker, assuming you are running etcd and Prometheus
as given in the examples here,

```bash
export ETCD_ADDRESS=http://192.168.99.100:2379
export PROMETHEUS_ADDRESS=http://192.168.99.100:9090

docker run -d -p 7070:7070 \
       -e ETCD_ADDRESS \
       -e PROMETHEUS_ADDRESS \
       squaremo/flux-web
```

You should now see the web interface on `http://192.168.99.100:7070/`.

### Prerequisites

Weave Flux assumes you have an etcd installation handy. If you don't,
it's easy to run one under Docker for the purpose of kicking the
tires. Assuming you have a host with Docker running and accessible on
`HOST_IP`, do

```bash
docker run -d -p 2379:2379 \
       -e "ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379" \
       -e "ETCD_ADVERTISE_CLIENT_URLS=http://$HOST_IP:2379" \
       quay.io/coreos/etcd
```

Now you have an etcd available on `http://$HOST_IP:2379`.

If you run the web interface, you will also need an instance of
Prometheus. See the [web interface README](web/README.md) for more
details.

### Disclaimer

Weave Flux is a work in progress. There are rough edges, and areas
where expedience has driven the design.
