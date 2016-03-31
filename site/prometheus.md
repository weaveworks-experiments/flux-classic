---
layout: page
title: Integrating with Prometheus
---

Flux has a two-part integration with [Prometheus][prom-site]: firstly,
the Flux daemon `fluxd` exposes metrics that Prometheus can scrape;
and secondly, the web dashboard will query those metrics to populate
its charts and gauges.

## Exposing stats to Prometheus from fluxd

To tell `fluxd` to expose stats for Prometheus, supply the
`--listen-prometheus` option, with a listening address (`:9000` is
fine).

The `bin/run-flux` script assumes this is what you want, and does it
for you.

## Running Prometheus

It's easy to run Prometheus under Docker; however, you will need some
way of telling Prometheus about all of the hosts running the fluxd, so
it knows to scrape them for stats. See below for some ways to do that.

### Configuring Prometheus

In the examples below, I've put the configuration in a file
`prometheus.yml` in the current directory, and mounted it into the
container filesystem in the place Prometheus expects it. I left the
`global` section empty, which will work, though you may of course have
your own configuration to add there.

#### Using hostnames or IP addresses

If you can supply a static list of hostnames or host IP addresses, you
can just put them in a stanza in the configuration file. For example,
using host names:

```yaml
global:

scrape_configs:
  - job_name: 'flux'
    scrape_interval: 5s
    scrape_timeout: 10s
    target_groups:
      - targets:
        - host-one:9000
        - host-two:9000
        - host-three:9000
```

Note the port numbers, which match whatever you told fluxd to listen
on with `--listen-prometheus` (or `9000` if you use `run-flux`).

You can then give the Prometheus container the IP addresses (and the
configuration, with a volume mount) when starting it:

```bash
docker run -d -p 9090:9090 \
       -v $PWD/prometheus.yml:/etc/prometheus/prometheus.yml \
       --add-host "host-one:192.168.99.101" \
       --add-host "host-two:192.168.99.102" \
       --add-host "host-three:192.168.99.103" \
       prom/prometheus:master
```

#### Using Prometheus's service discovery configs

Prometheus has a [handful of "service discovery" mechanisms][prom-sd],
which let you put a record of the hosts somewhere, which Prometheus
will poll.

For example, if you happen to be running all your containers on a
[Weave][weave-site] network, this can be as easy as making a DNS entry
for each host,

```bash
weave dns-add $(weave expose) weave -h flux.weave.local
```

and adding a `dns_sd_configs` stanza to the Prometheus configuration:

```yaml
global:

scrape_configs:
  - job_name: 'flux'
    scrape_interval: 5s
    scrape_timeout: 10s
    dns_sd_configs:
        - port: 9000
          type: A
          names:
            - 'flux.weave.local'
```

The `$(weave expose)` is needed to give the host an IP address on the
Weave network, since fluxd runs in the host's network namespace.

[prom-site]: https://github.com/prometheus/prometheus
[prom-sd]: http://prometheus.io/docs/operating/configuration/#scrape-configurations-scrape_config
[weave-site]: https://github.com/weaveworks/weave
