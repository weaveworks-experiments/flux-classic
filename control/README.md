# coatl
running, conducting, directing

The parts here are:

coatlctl - Command-Line Interface to set up and enrol services
listen - simple listener that just prints out when something happens.

These programs rely on a running `etcd` listening on port 2379.
To run `etcd`:

    docker run --name etcd -d -p 2379:2379 quay.io/coreos/etcd \
      -advertise-client-urls http://0.0.0.0:2379 \
      -listen-client-urls http://0.0.0.0:2379
