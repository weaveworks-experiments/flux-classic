---
layout: page
title: Flux overview
---

This page describes the main concepts needed to use Flux.

## Model

Weave Flux lets you define _services_.  A service has an IP address
and port.  These service addresses are _floating addresses_; they
don't correspond to any host.  When clients attempt to connect to one,
the Flux router (a.k.a. the balancer) transparently forwards the
connection to a service _instance_.  Connections are load balanced
over the available instances.

_instances_ correspond to Docker containers.  The containers are
automatically enrolled as service instances according to _selection
rules_ you supply.  For example, a selection rule might specify that
all containers with a particular image name become instances of a
corresponding service.

## Components

A running Flux deployment consists of

 1. an [agent](agent) on each host, which detects instances starting
 and stopping on that host;
 2. a [balancer](balancer) on each host, which proxies
 connections to services for clients on that host;
 3. Optionally, one or more [edge balancers](edgebal), which accept
 connections to services from the outside world.

To control and examine the state of your services, Flux provides a
command-line tool called [fluxctl](fluxctl). To monitor
the performance of the services, Flux has a [web dashboard](web).

All of the above are available as Docker images.

At present, Flux relies on [etcd][etcd-site] to store its
configuration and runtime data; and may be used [in
conjunction](prometheus) with [Prometheus][prometheus-site] to provide
runtime metrics for services.

[etcd-site]: https://github.com/coreos/etcd
[prometheus-site]: https://github.com/prometheus/prometheus
