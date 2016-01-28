---
layout: page
title: Docker agent
---

To integrate with Docker, Flux includes an agent that will enrol
containers as service instances, as they are started. The agent is run
on each Docker host. As containers are started, it will test them
against the service definitions, and add any containers that match. It
also removes containers when they die.

The agent is available as a Docker image called `weaveworks/flux-agent`.

## Operating the agent

The agent needs to be told an IP address, reachable from other hosts,
to give to containers that have mapped ports. This is supplied in the
`HOST_IP` environment entry. It also needs to be told how to contact
etcd: pass in an address in the `ETCD_ADDRESS` environment entry.

The agent needs to be able to connect to Docker. So it can do this,
bind-mount Docker's Unix domain socket (usually
`/var/run/docker.sock`) using `-v`; it's expected to be in the
container filesystem at `/var/run/docker.sock`.

All together, assuming `ETCD_ADDRESS` and `HOST_IP` are in the
environment already, a Docker command line looks like this:

```
docker run -d --name "fluxagent" \
       -e HOST_IP -e ETCD_ADDRESS \
       -v "/var/run/docker.sock:/var/run/docker.sock" \
       weaveworks/flux-agent
```

The `run-flux` script starts a Docker image using an appropriate
Docker command line.
