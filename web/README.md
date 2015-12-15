## Web interface

### Prerequisites

The web interface depends both on etcd (as covered in the main
README), and on [Prometheus][prom-site].

The essentials:

 * You are running etcd and Prometheus; and,
 * The Amber balancers are told to expose stats to Prometheus; and,
 * Prometheus knows how to connect to them.

To tell the balancer to expose stats for Prometheus, supply the
`--expose-prometheus` option, with a listening address (`:9000` is
fine). For example,

```bash
docker run -d --net=host --privileged \
       -e ETCD_ADDRESS \
       squaremo/ambergreen-balancer --expose-prometheus :9000
```

(The `run-amber` script assumes this is what you want, and does it for
you.)

### Running the web interface

The web interface needs to know how to connect to etcd (using the
environment entry `ETCD_ADDRESS`) and to Prometheus (using the
environment entry `PROM_ADDRESS`). To run under Docker, assuming you
are running etcd and Prometheus as given in the examples here,

```bash
export ETCD_ADDRESS=http://192.168.99.100:4001
export PROM_ADDRESS=http://192.168.99.100:9090

docker run -d -p 7070:7070 \
       -e ETCD_ADDRESS \
       -e PROM_ADDRESS \
       squaremo/ambergreen-web
```

You should now see the web interface on `http://192.168.99.100:7070/`.

### Running Prometheus

It's easy to run Prometheus under Docker; however, you will need some
way of telling Prometheus about all of the hosts running Amber, so it
knows to scrape them for stats. See just below for some ways to do
that.

### Configuring Prometheus

In the examples below, I've put the configuration in a file
`prometheus.yml` in the current directory, and mounted it into the
container filesystem in the place Prometheus expects it. I left the
`global` section empty, which will work, though you may of course have
your own configuration to add there.

#### Using hostnames or IP addresses

If you can supply a static list of hostnames or host IP addresses, you
can just put them in a stanza in a Prometheus configuration file. For
example, using host names:

```yaml
global:

scrape_configs:
  - job_name: 'amber'
    scrape_interval: 5s
    scrape_timeout: 10s
    target_groups:
      - targets:
        - host-one:9000
        - host-two:9000
        - host-three:9000
```

Note the port numbers, which match whatever you told the balancer to
listen on with `--expose-prometheus`.

You can then give the Prometheus container the IP addresses (and the
configuration, with a volume mount) when starting it:

```bash
docker run -d -p 9090:9090 \
       -v $PWD/prometheus.yml:/etc/prometheus/prometheus.yml \
       --add-host "host-one:192.168.99.101" \
       --add-host "host-two:192.168.99.102" \
       --add-host "host-three:192.168.99.103" \
       prom/prometheus:master
```

#### Using Prometheus's service discovery configs

Prometheus has a [handful of "service discovery" mechanisms][prom-sd], which let
you put a record of the hosts somewhere, which Prometheus will poll.

For example, if you happen to be running all your containers on a <a
href="">Weave</a> network, this can be as easy as making a DNS entry
for each host,

```bash
weave dns-add $(weave expose) weave -h amber.weave.local
```

and adding a `dns_sd_configs` stanza to the Prometheus configuration:

```yaml
global:

scrape_configs:
  - job_name: 'amber'
    scrape_interval: 5s
    scrape_timeout: 10s
    dns_sd_configs:
        - port: 9000
          type: A
          names:
            - 'amber.weave.local'
```

The `$(weave expose)` is needed to give the host an IP address on the
Weave network, since the balancer runs in the host's network
namespace.

[prom-sd]: http://prometheus.io/docs/operating/configuration/#scrape-configurations-scrape_config
[prom-site]: https://github.com/prometheus/prometheus
