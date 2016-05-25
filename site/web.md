---
title: Web Dashboard
menu_order: 80
---

Flux provides a view of your services' configuration and performance
with a web app. As long as it can reach etcd and prometheus, you can
run it anywhere in your infrastructure.

It is available as the Docker image `weaveworks/flux-web`.

To run it, you need to supply the following environment entries:

 - `ETCD_ADDRESS`: an etcd endpoint, e.g., `http://192.168.99.100:2379`
 - `PROMETHEUS_ADDRESS`: the URL of the Prometheus server, e.g.,
  `http://192.168.99.100:9090`

You will usually want to publish the port `7070` so you can access the
site from your browser; use something like `-p 7070:7070` with Docker
to map it to a host port.

Here is an example of running the image, which assumes you have those
environment entries above:

```
docker run -d -e ETCD_ADDRESS -e PROMETHEUS_ADDRESS -p 7070:7070 \
    --name=fluxweb weaveworks/flux-web
```


**See Also**

 * [Integrating with Prometheus](/site/prometheus.md)
