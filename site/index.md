---
layout: page
title: Weave Flux Documentation
---

Flux is a service routing layer that lets you control how containers
are accessed as services, without dictating how the containers are
created, placed or otherwise orchestrated.

Here are some uses of Flux:

* Allow service containers to be moved between hosts without
  restarting client containers
* Internal load-balancing of requests among containers
* Rolling upgrades and blue-green deployments of microservices
* When troubleshooting an issue in a microservice architectures,
  tracking down the microservice to blame
* Automatic configuration of an edge load balancer (currently nginx is
  supported)

Flux is not a platform, and does not require changes to your
application code. It can work with other Docker-based tooling, such as
container schedulers.  Flux works with Weave Net and other Docker
network plugins, but does not require them.

If you are new to Flux, please read on to <a
href="#introduction-to-flux">the introduction</a>. Otherwise, guides
and reference docs are linked from the contents to the side.

Flux is alpha software.  There may be rough edges, and it is still
evolving.  We are making preliminary releases in order to gather
feedback, so please let us know your thoughts. You can file an issue
on the [github repo](https://github.com/weaveworks/flux/), or contact
us on any of the channels mentioned on the [Weaveworks help
page](http://www.weave.works/help/).

## Introduction to Flux

Once upon a time, most web applications had a simple architecture: a
load balancer relayed requests to a single uniform tier of application
servers, which connected to a database.  Assembling the pieces of such
an architecture and troubleshooting any problems was relatively
straightforward.  These days, many projects are using microservices
instead.  But the benefits of microservices come at the cost of a more
complicated architecture.  Weave Flux aims to help tame this
complexity:

* When there's a problem, it can be hard to identify which
microservice is at fault.  Flux can show you information about the
requests between microservices, to help isolate problems.

* Flux provides lightweight client-side proxying, to load balance
requests between microservices.  This avoids the additional latency or
the configuration burden of using traditional load balancers for this
task.

* Flux is container-aware.  It integrates with Docker, and will
automatically reconfigure itself as containers are started and
stopped.

* Flux integrates with external load balancers, to leverage their rich
feature set for load balancing at the edge of your application
(i.e. of requests coming from the internet).  Currently nginx is
supported, with other well-known load balancers in the works.

* Flux helps with deploying new versions of services without downtime.
You can have different versions of a service running side-by-side, and
manage whether flux routes request to the new version, the old
version, or load balances across both.  And it helps you watch for
signs of trouble as you switch over.
