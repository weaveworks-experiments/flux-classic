---
layout: page
title: Flux overview
---

## Components

A running Flux deployment consists of

 1. an <a href="/agent/">agent</a> on each host, which detects
 instances starting and stopping on that host;
 2. a <a href="/balancer/">balancer</a> on each host, which proxies
 connections to services for clients on that host;
 3. <a href="/edgebal/">edge balancers</a> on some hosts, which proxy
 connections to services for the outside world.

To control and examine the state of your services, Flux provides a
command-line tool called <a href="/fluxctl/">`fluxctl`</a>. To monitor
the performance of the services, Flux has a <a href="/web/">web
dashboard</a>.

All of the above are available as Docker images.

At present, Flux relies on an installation of [etcd][etcd-site] to
store its configuration and runtime data; and may be used <a
href="/prometheus/">in conjunction</a> with
[Prometheus][prometheus-site] to provide runtime metrics for services.

[etcd-site]: https://github.com/coreos/etcd
[prometheus-site]: https://github.com/prometheus/prometheus
