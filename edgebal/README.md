# Edge balancer Docker image

This is a pre-baked image that packages nginx with an Weave Flux
listener which generates nginx configuration. The image entry point is
supervisord, which runs both the agent and nginx.

## What is its purpose

This image is mainly to demonstrate integration of Weave Flux with
nginx; but it may be useful as-is in some simple scenarios, and it is
adaptable to others.

Used as-is, it will expose the Weave Flux service you name in the
command line, on port 80, at `/`.

### Operating the edge balancer

The nginx process needs to be able to reach instances of the
service. Usually, this means you will need one of these situations:

 - the balancer and the instances are all on the same Docker network
   (e.g., a bridge network on a single host, or a Weave network) and
   you are using `--port-fixed` when selecting instances; or,
   
 - you are using `--port-mapped`, in which case each instance is
   addressed via a host IP.

You will also want to be able to reach the edge balancer from
"outside", which will most likely mean you need to publish a port for
it (`-p 8080:80`), or run it in the host network namespace
(`--net=host`). The latter will also require `--privileged`, since it
needs to bind to a privileged port (`80`).

### Trying it out

Provided you have the Weave Flux prerequisities in the form of an
endpoint for etcd in `ETCD_ADDRESS`, to expose the service `pages-svc`
you can do:

```bash
docker run -p 8080:80 -d -e ETCD_ADDRESS -e SERVICE=foo-svc \
       squaremo/flux-edgebal
```

To run in the host network namespace:

```bash
docker run --privileged --net=host -d \
       -e ETCD_ADDRESS -e SERVICE=foo-svc \
       squaremo/flux-edgebal
```

## Adapting the image

The main means of adaption is to replace the configuration template
(and probably the default configuration). These are the files
`/home/flux/nginx.tmpl` and `/home/flux/nginx.conf` in the
image filesystem, so you could, for instance, build a new image that
copies over them.
