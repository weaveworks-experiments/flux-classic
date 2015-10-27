# Load-balancer / Ambassador for microservices

Includes a layer 4 (TCP) and layer 7 (HTTP) load-balancer that can be
deployed on and controlled from multiple hosts. Presently,
co-ordination is done with etcd. The load-balancer acts as an
ambassador on each host, brokering connections with service instances.

Services and service instances may be managed through a command-line
tool.

It can be used with containerised deployments, and has Docker
integration for this purpose.

 * [Load balancer](balancer/README.md)
 * [Command-line tool](command/README.md)
 * [Docker integration](agent/README.md)
