---
layout: page
title: Getting started with Flux
---

This is a guide to getting a minimal Flux system running. It assumes
only that you have a host running Docker -- to get to that point, you
can [install Docker], or use [docker-machine] to create a VM on which
to run it.

We'll take some shortcuts here, in particular by using prepared images
for some of the prerequisites. This is just so we can go quickly to
"kicking the tires".

## Prerequisites

First we'll set up a couple of environment variables for things we'll
use repeatedly. We need a host IP address for whatever is running
Flux, and an address for things to find `etcd` on. The host IP needs
to be accessible from inside Docker containers, so `localhost` (or
`127.0.0.1`) won't do; the IP address assigned to your host on the
local network will do the trick, or if you're using Docker on a VM,
the address assigned to that VM.

```sh
$ export HOST_IP=192.168.3.165
# or if using docker-machine, something like
# export HOST_IP=$(docker-machine ip flux)
```

OK, now let's run the two bits of infrastructure we need: etcd and
Prometheus. We'll run them in containers, of course.

```sh
$ docker run --name=etcd -d -p $HOST_IP::2379 quay.io/coreos/etcd \
       --listen-client-urls http://0.0.0.0:2379 \
       --advertise-client-urls=http://localhost:2379
# ...
$ export ETCD_ADDRESS=http://$(docker port etcd 2379)

# And run our pre-baked image of Prometheus
$ docker run --name=prometheus -d -e ETCD_ADDRESS -p $HOST_IP::9090 \
       weaveworks/flux-prometheus-etcd
# ...
$ export PROMETHEUS_ADDRESS=http://$(docker port prometheus 9090)
```

Now we have both etcd and prometheus, and (in the environment entries)
what we need to tell Flux so it can reach them.

## Starting Flux

Flux includes a script to start and stop the Flux components, but
we're going to do it by hand here so we can see what are the moving
parts. It's not much more complicated.

First, the agent:

```sh
$ docker run --name=fluxagent -d -e ETCD_ADDRESS \
       -v /var/run/docker.sock:/var/run/docker.sock \
       weaveworks/flux-agent --host-ip $HOST_IP
```

The agent needs to know where etcd is, and needs to be able to connect
to the Docker socket, to detect containers starting and stopping. It
also needs to know what IP address containers will be reachable on --
in this case, the host IP from before.

Now, the balancer:

```sh
$ docker run --name=fluxbalancer -d -e ETCD_ADDRESS \
       --net=host --cap-add=NET_ADMIN \
       weaveworks/flux-balancer \
       --listen-prometheus=:9000 \
       --advertise-prometheus=$HOST_IP:9000
```

The balancer needs to run in the host's network namespace
(`--net=host`), and to have the `NET_ADMIN` capability so it can use
iptables. We also tell it to serve prometheus metrics
(`--listen-prometheus`) and supply the address on which those will be
reachable (`--advertise-prometheus`, again with the host IP address).

Flux is now running, and you can check this in Docker:

```sh
$ docker ps
CONTAINER ID        IMAGE                             COMMAND                  CREATED             STATUS              PORTS                                                         NAMES
64a9b5cf4290        weaveworks/flux-balancer          "/home/flux/server --"   3 seconds ago       Up 3 seconds                                                                      fluxbalancer
893f7e238473        weaveworks/flux-agent             "/bin/dlisten --host-"   22 seconds ago      Up 21 seconds                                                                     fluxagent
# ...
```

## Trying it out

Now we'll actually use Flux! Once it's running, you control Flux with
`fluxctl`; this is available as a Docker image. We'll drive it using
an `alias`, to avoid typing the `docker run ...` bit again and again:

```sh
$ alias fluxctl="docker run --rm -e ETCD_ADDRESS weaveworks/flux-fluxctl"
```

We'll start by creating a service `hello`, which will represent some
hello-world containers we'll run presently.

```sh
$ fluxctl service hello --image weaveworks/hello-world --port-mapped 80
```

The `--port-mapped` bit says our hello-world containers will have a
port mapped to `80`, and that's how they should be reached.

Let's start some of those containers.

```sh
$ docker run -d -P weaveworks/hello-world
# ...
$ docker run -d -P weaveworks/hello-world
# ...
$ docker run -d -P weaveworks/hello-world
# ...
```

Now we can see if Flux has noticed them:

```sh
$ fluxctl info
hello
  RULES
    default {"image":"weaveworks/hello-world"}
  INSTANCES
    ccc5ed490a6a6d7d7ee4381857da752f38d995eb08a9a60f19ec946599d76511 192.168.1.129:32770 live
    3b84320f7958a7159f0d172b8408bb6953f8dfa49e425708377de34c9a97ee08 192.168.1.129:32771 live
    eba9d0a13a2d6d735a47f2b4e32ead6f6e15e11400b96b15ab566f325fd0574d 192.168.1.129:32772 live
```

Yes! It did.
