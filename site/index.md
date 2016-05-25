---
title: About Weave Flux
menu_order: 10
---

Weave Flux is a service routing layer for containers.

At one time, most web applications had a simple architecture: a
load balancer relayed requests to a single uniform tier of application
servers that connected to a database.  Assembling the pieces of such
an architecture and troubleshooting any problems was relatively
straightforward.

###Taming Microservices Complexity

These days, many projects are using microservices instead.  But the
benefits of microservices come at the cost of a more complicated
architecture.  Flux aims to help tame this complexity.

*When there's a problem, it can be hard to identify which
microservice is at fault*. Flux shows you information about the
requests between microservices, to help you isolate problems.

*Flux provides lightweight client-side proxying, to load balance
requests between microservices*.  This avoids the additional latency or
the configuration burden of using traditional load balancers for this
task.

*Flux is container-aware*.  It integrates with Docker, and will
automatically reconfigure itself as containers are started and
stopped.

With Flux's service routing layer, you can:

* Gracefully replace the containers implementing a service without
  needing to restart clients

* Do rolling upgrades and blue-green deployments of microservices

* Automatically configure an ingress load balancer (currently Nginx is
  supported)

Flux is not a platform, and does not require changes to your
application code. It works with other Docker-based tooling, such as
container schedulers.  Flux works with Weave Net and other Docker
network plugins, but does not require them.

If you are new to Flux, try the [getting started
guide](/site/getstarted.md) or read the technical
[overview](/site/overview.md).  Otherwise, other guides and reference
docs are linked from the contents to the side.

Flux is alpha software.  There may be rough edges, and it is still
evolving.  We are making preliminary releases in order to gather
feedback, so please let us know your thoughts. You can file an issue
on the [github repo](https://github.com/weaveworks/flux/), or contact
us on any of the channels mentioned on the [Weaveworks help
page](http://www.weave.works/help/).
