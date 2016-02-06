---
layout: page
title: Docker agent
---

To integrate with Docker, Flux includes an agent that will enrol
containers as service instances, as they are started. The agent is run
on each Docker host. As containers are started, it will test them
against the service definitions, and add any containers that match. It
also removes containers when they die.

The agent is available as a Docker image called
`weaveworks/flux-agent`.

## Operating the agent

The agent needs to know how to extract an address from a container, so
that balancers can reach the container. Minimally, it needs to be told
the IP address of the host it's running on. The host IP address must
be reachable from other hosts. This is supplied in the `HOST_IP`
environment entry, or the `--host-ip` argument.

The agent also needs to be told how to contact etcd: pass in an
address in the `ETCD_ADDRESS` environment entry.

Lastly, the agent needs to be able to connect to Docker to get
information about containers. So it can do this, bind-mount Docker's
Unix domain socket (usually `/var/run/docker.sock`) using `-v`; it's
expected to be in the container filesystem at `/var/run/docker.sock`.

Assuming `ETCD_ADDRESS` and `HOST_IP` are in the environment already,
a Docker command to start the agent looks like this:

```
docker run -d --name "fluxagent" \
       -e HOST_IP -e ETCD_ADDRESS \
       -v "/var/run/docker.sock:/var/run/docker.sock" \
       weaveworks/flux-agent
```

The `run-flux` script effectively wraps this minimal command line.

### More on container addresses

By default, the agent will assume you are publishing ports, and
extract an address from each container by using the host port that
Docker maps to the container's port, and the host IP.

If you're using Weave Net, or your containers are otherwise able to
connect to each other across hosts, you can tell the agent this with a
`--network-mode=global` argument (`--network-mode=local` is the
default).

In this case, the agent will look in the container's network settings
to find an IP address, and use the port given for the service.

A special case is if you run a container in the host's networking
namespace (using `--net=host`). The agent will use the host IP address
it was given along with the service port, disregarding the network
mode.
