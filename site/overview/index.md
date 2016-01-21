---
layout: page
title: Flux overview
---

## Components

A running Flux deployment consists of

 1. an agent on each host, which detects instances starting and
 stopping on that host;
 2. a balancer on each host, which proxy connections to services for
 clients on that host;
 3. edge balancers on some hosts, which proxy connections to services for the outside world.

All of the above are available as Docker images.

At present, Flux relies on an installation of [etcd][etcd-site] to
store its configuration and runtime data; and may be used in
conjunction with [Prometheus][prometheus-site] to provide runtime
metrics for services.

[etcd-site]: https://github.com/coreos/etcd
[prometheus-site]: https://github.com/prometheus/prometheus
