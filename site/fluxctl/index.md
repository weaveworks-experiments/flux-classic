---
layout: page
title: fluxctl command-line interface
---

`fluxctl` is the command-line interface to Weave Flux. It has
subcommands for defining services, selecting containers to be
instances of a service, and querying the state of the system.

Synopsis:

```
Usage:
  fluxctl [command]

Available Commands:
  service     define a service
  list        list the services defined
  query       display instances selected by the given filter
  rm          remove service definition(s)
  select      add a selection rule to a service
  deselect    remove a selection rule from a service

Flags:
  -h, --help[=false]: help for fluxctl
```

### Define and remove services

`fluxctl service` is the subcommand to define a service. It needs a
name, and usually you'll supply the address on which the service
should listen.

You can specify the protocol for the service -- whether it should be
treated as HTTP or plain TCP -- in the address, or with another
option. (Using HTTP means you get extra, HTTP-specific metrics.)

It's possible to create a service that has no address. You might do
this if you were going to use it only to control an external load
balancer (like [the edgebal image](../edgebal/)).

There are also options for selecting containers to be instances, as a
shortcut to using a subsequent `fluxctl select ...` command.

```
Usage:
  fluxctl service <name> [flags]

Flags:
      --address="": in the format <ipaddr>:<port>[/<protocol>], the IP address and port at which the service should be made available on each host; optionally, the protocol to assume.
      --env="": select only containers with these environment variable values, given as comma-delimited key=value pairs
      --image="": select only containers with this image
      --labels="": select only containers with these labels, given as comma-delimited key=value pairs
      --port-fixed=0: Use a fixed port, and get the IP address from docker network settings
      --port-mapped=0: Use the host IP address, and the host port mapped to the given container port
  -p, --protocol="": the protocol to assume for connections to the service; either "http" or "tcp". Overrides the protocol given in --address if present.
      --tag="": select only containers with this tag
```

You can remove a service, or all services, with `fluxctl rm`:

```
Usage:
  fluxctl rm <service>|--all
```

### Select and deselect instances

Once you have a service defined, you can select containers to be
enrolled as instances of the service. Weave Flux will load-balance
connections to the *service address* amongst the *instance addresses*.

Selecting containers is done by giving a rule for matching properties
of a given container; the container is enrolled if _all_ the
properties match. For example, if the rule is
`image=foo-api,tag=v0.3`, then a container must have both the image
`foo-api` and the tag `v0.3` to be included.

In general the rules match labels (`--labels`) and environment entries
(`--env`) of the container. The special labels `image` and `tag` match
the image name and image tag respectively (`foo-api` and `v0.3` of the
image `foo-api:v0.3`). These have their own options `--image` and
`--tag`.

When you select containers, you must also say how to connect to
them. There are two alternatives: using mapped ports, or assuming a
common network. The corresponding flags are:

 * `--port-mapped <port>`, which means use the host's IP address,
   along with the host port that is mapped to the given container
   port. This is for when you are mapping ports on the host using `-p`
   or `-P` with `docker run ...`.

 * `--port-fixed <port>` which means use the IP address reported by
   Docker (i.e., as from `docker inspect ...`), along with the given
   port. This is for when your containers have a network connecting
   them (e.g., if you are using a Weave network) and don't need to map
   ports.

A service may have several rules, e.g., from more than one invocation
of `fluxctl select`; a container will be enrolled if it matches _any_
of the rules. To repeat: matching _any_ rule will do, but _each_part_
of the rule must match.

```
Usage:
  fluxctl select <service> <rule> [flags]

Flags:
      --env="": select only containers with these environment variable values, given as comma-delimited key=value pairs
      --image="": select only containers with this image
      --labels="": select only containers with these labels, given as comma-delimited key=value pairs
      --port-fixed=0: Use a fixed port, and get the IP address from docker network settings
      --port-mapped=0: Use the host IP address, and the host port mapped to the given container port
      --tag="": select only containers with this tag
```

When you use `fluxctl select ...`, you give the rule a name. The name
can be used to remove that rule later. A container that matched the
removed rule may remain as an instance, if it matches another rule.

```
Usage:
  fluxctl deselect <service> <rule>
```

### List services and query instances

You can list the currently configured services, and optionally their
selection rules, using `fluxctl list`.

```
Usage:
  fluxctl list [flags]

Flags:
  -f, --format="": format each service with the go template expression given
      --format-rule="": format each rule with the go template expression given (implies --verbose)
  -v, --verbose[=false]: show the list of selection rules for each service
```

You can also query for instances, of a particular service or of any
service, using `fluxctl query`.

This subcommand accepts the same label-matching flags as select, and
will display only the instances that match.

```
Usage:
  fluxctl query [flags]

Flags:
      --env="": select only containers with these environment variable values, given as comma-delimited key=value pairs
  -f, --format="": format each instance according to the go template given
      --image="": select only containers with this image
      --labels="": select only containers with these labels, given as comma-delimited key=value pairs
  -s, --service="": print only instances in <service>
      --tag="": select only containers with this tag
```

By default, `fluxctl query` will print the IDs of matching instances,
one to a line. You can supply a template expression to format the
instance data on each line; for example,

```
fluxctl query --format '{{json .}}'
```
