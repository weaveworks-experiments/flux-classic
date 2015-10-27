# dlisten
Listens to Docker and enrols/unenrols containers matching services registered in etcd

Configuration environment variables:

- `DOCKER_HOST` - default `unix:///var/run/docker.sock`
- `ETCD_ADDRESS` - default `http://127.0.0.1:4001`

Each time a Docker container starts, `dlisten` inspects it and enrolls
it in a coatl service:

- if the container's image tag matches a service `--docker-image`
- if the container has an environment variable or label `SERVICE_NAME`
- if the container's name matches a service name, then that is used

The port number is:

 - if the container docker-exposes exactly one port, then that is taken
 - if it has an environment variable or label `SERVICE_PORT`, then that overrides
 - otherwise, the port configured for the service

To run inside a Docker container:

    docker build -t bboreham/dlisten .
    docker run -d -e ETCD_ADDRESS=http://etcd:4001 -v /var/run/docker.sock:/var/run/docker.sock dlisten
