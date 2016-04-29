---
layout: page
title: The Flux daemon, fluxd
---

The Flux daemon (`fluxd`) runs on each host, and does two things:

 - it watches Docker containers start and die, so it can keep track of
   which containers are service instances;
 - it proxies connections to services, for any clients on the host

`fluxd` is available as the Docker image `weaveworks/fluxd`.

## Operating the daemon

The daemon needs to know how to extract an address from a container,
so that other daemons can reach the container when proxying
connections. Minimally, it needs to be told the IP address of the host
it's running on. The host IP address must be reachable from other
hosts. This is supplied in the `HOST_IP` environment entry, or the
`--host-ip` argument.

The daemon also needs to be told how to contact etcd: pass in an
address in the `ETCD_ADDRESS` environment entry.

The daemon needs to be able to connect to Docker to get information
about containers. So it can do this from its own container, bind-mount
Docker's Unix domain socket (usually `/var/run/docker.sock`) using
`-v`; it's expected to be in the daemon's filesystem at
`/var/run/docker.sock`.

The daemon needs to run using the host's network stack, and with the
`NET_ADMIN` capability (or simply privileged).

### Example of running fluxd

Assuming `ETCD_ADDRESS` and `HOST_IP` are in the environment already,
a Docker command to start the daemon looks like this:

```
docker run -d --name "fluxd" --cap-add=NET_ADMIN --net=host \
       -e HOST_IP -e ETCD_ADDRESS \
       -v "/var/run/docker.sock:/var/run/docker.sock" \
       weaveworks/fluxd
```

The script `bin/run-flux` is essentially a wrapper for this
invocation.

### More on container addresses

By default, the daemon will assume you are publishing ports, and
extract an address from each container by using the host port that
Docker maps to the container's port, and the host IP.

If you're using Weave Net, or your containers are otherwise able to
connect to each other across hosts without port mapping, you can tell
the daemon this with a `--network-mode=global` argument
(`--network-mode=local` is the default).

In this case, the daemon will look in the container's network settings
to find an IP address, and use the port given in the _service_'s
address.

A special case is if you run a container in the host's networking
namespace (using `--net=host`). The daemon will use the host IP
address it was given along with the service port, disregarding the
network mode.

### Exposing metrics to Prometheus

The daemon exposes a handful of metrics for the connections it
proxies.

| Metric | Description |
|--------|-------------|
| flux_connections_total | A counter of the TCP connections proxied by the daemon |
| flux_http_total | A counter of the HTTP requests proxied |
| flux_http_roundtrip_usec | A summary of HTTP roundtrip times, in microseconds |
| flux_http_total_usec | A summary of HTTP total transaction time, in microseconds |

### Daemon command-line reference

```
Usage of fluxd:
  -advertise-prometheus string
    	IP address and port to advertise to Prometheus; e.g. 192.168.42.221:9000
  -bridge string
    	bridge device (default "docker0")
  -chain string
    	iptables chain name (default "FLUX")
  -debug
    	output debugging logs
  -host-ip string
    	IP address for instances with mapped ports
  -host-ttl int
        The daemon will give its records this time-to-live in seconds, and refresh them while it is running (default 30)
  -listen-prometheus string
    	listen for connections from Prometheus on this IP address and port; e.g., :9000
  -network-mode string
    	Kind of network to assume for containers (either "local" or "global") (default "local")
```
