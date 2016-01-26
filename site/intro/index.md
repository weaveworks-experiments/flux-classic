---
layout: page
title: Introduction
---

Once upon a time, most web applications had a simple architecture: a
load balancer relayed requests to a single uniform tier of application
servers, which connected to a database.  The benefits of microservices
come at the cost of a more complicated architecture.  Weave Flux aims
to help tame this complexity:

* When there's a problem, it can be hard to identify which
microservice is at fault.  Flux can show you information about the
requests between microservices, to help isolate problems.

* Flux provides lightweight internal load balancing, to load balance
requests between microservices.  This avoids the additional latency or
the configuration burden of using traditional load balancers for this
task.

* Flux integrates with external load balancers, to leverage their rich
feature set for load balancing at the edge of your application
(i.e. of requests coming from the internet).  Currently nginx is
supported, with other well-known load balancers in the works.

* Flux is container-aware.  It integrates with Docker, and will
automatically reconfigure itself as containers are started and
stopped.

* Flux helps with deploying new versions of services without downtime.
You can have different versions of a service running side-by-side, and
manage whether flux routes request to the new version, the old
version, or load balances across both.  And it helps you watch for
signs of trouble as you switch over.

Flux is not a platform, and does not require changes to your
application code. It can work with other Docker-based tooling.  It is
agnostic about what Docker network plugin you use, if any.
