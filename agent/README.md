# Agent

A Docker listener that registers service instances when containers
start.

Running as a container:

```bash
docker run -d -e ETCD_ADDRESS=http://etcd:4001 \
       -v /var/run/docker.sock:/var/run/docker.sock \
       flux/agent
```

The listener extracts the IP address of a container from `docker
inspect`, so when using Weave it's important to use via the weave API
proxy, so that the IP addresses are those supplied by weave:

```bash
docker run -d -e ETCD_ADDRESS=http://etcd:4001 \
       -v /var/run/weave/weave.sock:/var/run/docker.sock \
       flux/agent
```
