# Edge balancer Docker image

This is a pre-baked image that packages nginx with an Ambergreen
listener which generates nginx configuration. The image entry point is
supervisord, which runs both the agent and nginx.

## What is its purpose

This image is mainly to demonstrate integration of Ambergreen with
nginx; but it may be useful as-is in some simple scenarios, and it is
adaptable to others.

Used as-is, it will expose the Ambergreen service you name in the
command line, on port 80, at `/`.

### Adapting the image

The main means of adaption is to replace the configuration template
(and probably the default configuration). These are the files
`/home/ambergreen/nginx.tmpl` and `/home/ambergreen/nginx.conf` in the
image filesystem, so you could, for instance, build a new image that
copies over them.
