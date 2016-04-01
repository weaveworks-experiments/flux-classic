---
layout: page
title: Using Weave Flux with Docker Swarm
---

This is a step-by-step guide to using Weave Flux with [Docker
Swarm](https://www.docker.com/products/docker-swarm).  We'll use Swarm
because it spreads containers across a cluster of machines, while
retaining a developer experience that is very close to that of the
plain Docker Engine on a single host.  So it should be possible to
follow this guide if you are familiar with Docker, even if you don't
know Swarm.  And these instructions are not hugely Swarm-specific, so
some parts will be relevant to using Flux in other contexts.

This guide shows the full details of the commands needed to create a
Docker Swarm cluster and deploy Flux to it, before using Flux.  If the
process seems onerous, skip to the section [_A simple service
example_](#a-simple-service-example) to see what the typical use of
Flux involves.  If you are deploying Flux on a regular basis, we
recommend that you script or otherwise automate the relevant steps.

* ToC
{:toc}

## Preliminaries

First, we'll create a Swarm cluster.  We'll use [Docker
Machine](http://www.docker.com/products/docker-machine) to do
this. Docker Machine is a tool that makes it simple to spin up docker
host VMs, and it has some built-in support for Swarm.  See Docker's
[instructions on creating a Swarm development
cluster](https://docs.docker.com/swarm/install-manual/) for more
details.  Here we'll create a modest cluster of two VMs (replacing
`$driver` with the [Docker Machine
driver](https://docs.docker.com/machine/drivers/) you wish to use):

```sh
$ cluster_id=$(docker run --rm swarm create)
$ docker-machine create -d $driver --swarm --swarm-master \
        --swarm-discovery token://$cluster_id swarm-master
Running pre-create checks...
Creating machine...
[...]
Configuring swarm...
Checking connection to Docker...
Docker is up and running!
To see how to connect Docker to this machine, run: docker-machine env swarm-master
$ docker-machine create -d $driver --swarm \
        --swarm-discovery token://$cluster_id swarm-node-0
Running pre-create checks...
Creating machine...
[...]
```

Then we set the environment so the `docker` client command talks to
our Swarm cluster, rather than the local Docker Engine, and check that
it is working:

```sh
$ eval $(docker-machine env --swarm swarm-master)
$ docker run --rm hello-world

Hello from Docker.
[...]

```

## Deploying Flux

In this section, we'll deploy the basic Flux components to the Swarm
cluster.

etcd is a prerequisite for Flux, so first we start it in a container.
This is just a trivial single-node etcd deployment; if you have a
production etcd deployment already, you can set `$ETCD_ADDRESS` to
point to it instead.

```sh
$ docker run --name=etcd -d -p 2379:2379 quay.io/coreos/etcd -listen-client-urls http://0.0.0.0:2379 -advertise-client-urls=http://localhost:2379
a43f43b6f2958a3143a7c15643b42329768551b87858e965ab2f64b30ce8ac2d
$ export ETCD_ADDRESS=http://$(docker port etcd 2379)
```

Next we start the fluxd, the Flux daemon (see the
[Overview](overview)).  Fluxd must be run on each host, so we ask
`docker-machine` to list the hosts and use a Swarm scheduling
constraint to run an agent on each.

```sh
$ hosts=$(docker-machine ls -f {% raw %}'{{.Name}}'{% endraw %})
$ for h in $hosts ; do \
        docker run -d -e constraint:node==$h -e ETCD_ADDRESS \
            --net=host --cap-add=NET_ADMIN \
            -v /var/run/docker.sock:/var/run/docker.sock \
            weaveworks/fluxd -host-ip $(docker-machine ip $h) ; \
  done
6004ddd81bbcf01cb8fa4214546ad12c198fc96dccdd8f0573f9583bc25d9a79
df705d7e3a3c8b7a1c4e95b9e6c2d005f9bae2088155ac637f0d88802318b014
```

You have now deployed Flux!

## A simple service example

In this section, we'll define a service consisting of some Apache
httpd containers, and then send requests to the service with curl.  In
practical use, clients and service instances connected by Flux are
more likely to be microservices within an application.  But httpd and
curl provides a simple way to demonstrate the basic use of Flux.

Flux is administered using a command-line tool called `fluxctl`.
We'll be running `fluxctl` in a container, and because we will be
using it a few times, we'll define a variable `$fluxctl` so we don't
have to keep repeating the necessary `docker run` arguments:

```sh
$ fluxctl="docker run --rm -e ETCD_ADDRESS weaveworks/fluxctl"
```

First, we'll use the `fluxctl service` subcommand to
define a _service_.  Services are the central abstraction of Flux.

```sh
$ $fluxctl service hello --address 10.128.0.1:80 --protocol http
```

Here, we have defined a service called `hello`.  The `--address
10.128.0.1:80` option assigns that IP address and port to the
service. This is a _floating address_; it doesn't correspond to any
host, but when clients attempt to connect to it, their connections will
be transparently forwarded to a service instance (so you should ensure
that the addresses you assign to services do not correspond to any
real IP addresses in your network environment).  The `--protocol http`
option tells Flux that connections to this service will carry HTTP
traffic, so that it can extract HTTP-specific metrics.

Next, we'll start a couple of `hello-world` containers (note that we
need to use the `-P` option to `docker run` to make the ports exposed
by the container accessible from all machines in the cluster):

```sh
$ docker run -d -P weaveworks/hello-world
$ docker run -d -P weaveworks/hello-world
```

Flux does not yet know that these containers should be associated with
the service.  We tell it that by defining a _selection rule_, using
the `fluxctl select` subcommand:

```sh
$ $fluxctl select hello default --image weaveworks/hello-world
```

This specifies that containers using the Docker image `httpd` should
be associated with the `httpd` service. Connections to the service
will be forwarded to port 80 of the container, since that's the port
given in the service address (it's also possible to supply a different
container port when defining the service).

We can see the result of this using the `fluxctl info` subcommand:

```sh
$ $fluxctl info
HOSTS
192.168.42.149
192.168.42.202

SERVICES
hello
  RULES
    default {"image":"weaveworks/hello-world"}
  INSTANCES
    e96c0f2536630d1acec823bf9450474fcc5e3671eb571639be96409f230d9779 192.168.42.149:32768 live
    45f70013ae1d123d30e9a6eb6f975d594d978909206e7b1fd44cc35df490fa15 192.168.42.202:32769 live
```

Now we'll use `curl` to send a request to the service:

```sh
$ docker run --rm tutum/curl curl -s http://10.128.0.1/
<html>
  <head>
    <title>Hello from 45f70013ae1d</title>
  </head>
...
```

Flux load-balances requests across the service instances, so this
request might have been served by either httpd container.

## The Flux web UI

Flux features a web-based UI.  This section explains how to get
started with it.

The Prometheus time series database is a prerequisite for the UI.  So
first we start it in a container:

```sh
$ docker run --name=prometheus -d -e ETCD_ADDRESS -P weaveworks/flux-prometheus-etcd
```

(The `flux-prometheus-etcd` image is a version of Prometheus
configured to integrate with the other components of Flux via etcd.)

Next we start the Flux web UI, telling it how to connect to Prometheus:

```sh
$ export PROMETHEUS_ADDRESS=http://$(docker port prometheus 9090)
$ docker run --name=fluxweb -d -e ETCD_ADDRESS -e PROMETHEUS_ADDRESS -P \
    weaveworks/flux-web
```

Now we can point a browser to the address given by `docker port
fluxweb 7070` in order to view the UI:

<img src="images/swarm-ui.jpg" alt="Flux UI" width="800" height="583"/>

Here we see information about the service, including the instances
associated with it.  By selecting some instances, the UI will show
a chart of their request rates on the service.

We can use the `curl` container image to produce a stream of requests
to the service:

```sh
$ docker run --rm tutum/curl sh -c 'while true ; do curl -s http://10.128.0.1/ >/dev/null ; done'
```

Then view the request rates as a chart:

<img src="images/swarm-ui-chart.jpg" alt="Flux UI" width="800" height="583"/>

If we change the URL used in the curl command to one that does not
exist, the chart shows the change in HTTP status code indicating the
error:

<img src="images/swarm-ui-chart-nosuch.jpg" alt="Flux UI" width="800" height="583"/>

## Integrating load balancers

Here, we will deploy a load balancer to make our service accessible
outside our application.

So far we have set Flux up to route traffic internal to our
application. However, for most applications you will want at least one
service -- for example, a public API service -- to be accessible from
outside the application. The usual way to do this is by configuring an
HTTP proxy, like Nginx or HAProxy, to act as a load balancer (and
possibly a reverse proxy too).

Flux can dynamically reconfigure an HTTP load balancer, and includes a
pre-baked image to show how it's done. We'll use it to make our
"httpd" service available in a browser.

Since we want this to be available from outside, we will need a stable
address. We'll use our swarm master and its IP address, arbitrarily,
and map to a _specific_ port.

The pre-baked image is supplied with an address for etcd, and the name
of the service to load balance:

```sh
$ docker run  -p 8080:80 -d -e ETCD_ADDRESS -e SERVICE=httpd \
    weaveworks/flux-edgebal
```

Now you should be able to visit `http://$(docker-machine ip
swarm-master):8080/` in a browser. If you start or stop `httpd`
containers, Nginx will be told to load a new configuration.
