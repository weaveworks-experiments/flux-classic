---
layout: page
title: Integrating with Prometheus
---

Flux has a two-part integration with
[Prometheus](https://github.com/prometheus/prometheus): firstly, the
Flux daemon `fluxd` exposes metrics that Prometheus can scrape; and
secondly, the web dashboard will query those metrics to populate its
charts and gauges.

## Exposing stats to Prometheus from fluxd

Prometheus needs some way discover all of the hosts running fluxd, so
that it can probe them for metrics.  The docker image
`weaveworks/flux-prometheus-etcd` provides a Prometheus server that is
customized to automatically discover `fluxd` instances via etcd.  You
don't need to supply any options to `fluxd` to enable the integration
with Prometheus, although you can customize the port number on which
`fluxd` listens for connections from Prometheus with the
`--listen-prometheus` option (it defaults to port 9000).

Apart from the enhancements to support discovery via etcd,
`weaveworks/flux-prometheus-etcd` is just a plain Prometheus server.
If you already have a Prometheus server deployed, you can use that.
Though you'll need to arrange to use one of Prometheus' service
discovery mechanisms for it to scrape metrics from all hosts running
`fluxd`.


