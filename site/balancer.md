---
layout: page
title: Client-side load balancer
---

Flux includes a client-side load balancer, for intermediating access
to services. This is run on each host, and proxies internal traffic --
that is, connections to services originating in your own application,
rather than from the internet -- load-balancing them across the
services' instances.

The load balancer is available as a Docker image called
`weaveworks/flux-balancer`.

## Operating the load balancer

The load balancer image needs to run using the host's network stack,
and with the `NET_ADMIN` capability (or simply privileged). It also
needs the address of the shared store; and, optionally, the address on
which to expose its prometheus metrics.

Here is an example of running the balancer, which assumes you have
`ETCD_ADDRESS` (the URL for connecting to etcd) and `HOST_IP` (an IP
address reachable by your other hosts) in the environment:

```
docker run -d -e ETCD_ADDRESS --cap-add=NET_ADMIN --net=host \
       weaveworks/flux-balancer \
       --listen-prometheus :9000 --advertise-prometheus $HOST_IP:9000
```

The `run-flux` script starts the balancer and the [agent](/agent/)
images using appropriate Docker commands, given the environment
entries mentioned above.

### Prometheus metrics

The balancer exposes a handful of metrics for the connections it
proxies.

| Metric | Description |
|--------|-------------|
| flux_connections_total | A counter of the TCP connections made via the broker |
| flux_http_total | A counter of the HTTP requests made via the broker |
| flux_http_roundtrip_usec | A summary of HTTP roundtrip times, in microseconds |
| flux_http_total_usec | A summary of HTTP total transaction time, in microseconds |

### Balancer command-line options

There are a few command-line options that will adapt the behaviour of
the balancer.

```
Usage:

  -advertise-prometheus string
    	IP address and port to advertise to Prometheus; e.g. 192.168.42.221:9000
  -bridge string
    	bridge device (default "docker0")
  -chain string
    	iptables chain name (default "FLUX")
  -debug
    	output debugging logs
  -listen-prometheus string
    	listen for connections from Prometheus on this IP address and port; e.g., :9000
```

Of these, you will most likely only even need to supply `-listen-prometheus` and `-advertise-prometheus`. `-bridge` may be useful if you are using an interesting 

You can also add `--debug` as an option to get more detailed log
output.
