# Weave Flux – Microservices Routing

Weave Flux provides a simple way to assemble microservices.  It gives
you a decentralized service router that makes the health and
performance of your services visible.  Flux integrates with Docker,
and lets you use your favorite container scheduler.  All of this is
controlled from a simple CLI, with a web-based UI to examine the
behaviour of your services.

Here are some uses of Flux:

* Allow service containers to be moved between hosts without
  restarting client containers
* Internal load-balancing of requests among containers
* Rolling upgrades and blue-green deployments of microservices
* When troubleshooting an issue in a microservice architecture,
  tracking down the microservice to blame
* Automatic configuration of an edge load balancer (currently nginx is
  supported)

Flux is alpha software.  There may be rough edges, and it is still
evolving.  We are making preliminary releases in order to gather
feedback, so please let us know your thoughts. You can file an issue
on here, or contact us on any of the channels mentioned on the
[Weaveworks help page](http://www.weave.works/help/).

## Documentation

Full documentation, including instructions for installation and use,
is on the Flux website at
[weaveworks.github.io/flux/](http://weaveworks.github.io/flux/).

## Developing Flux

If you wish to work on Flux itself, you'll need to have [Docker
Engine](https://docs.docker.com/engine/installation/) and GNU make
installed.  The build process runs inside a container, and all
prerequisites are managed through the Dockerfiles.  To build, do:

```sh
$ make
```

This will produce several container images in the local docker daemon
(called `weaveworks/flux-...`).  It also saves the images as tar
archives under `docker/`, for loading into another docker daemon.

You can run the test suite by doing

```sh
$ make test
```
