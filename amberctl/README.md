# amberctl

`amberctl` is the command-line interface to Ambergreen. It has
subcommands for defining services, selecting containers, and querying
the state of the system.

Synopsis:

```
Usage:
  amberctl [command]

Available Commands:
  service     define a service
  list        list the services defined
  query       display instances selected by the given filter
  rm          remove service definition(s)
  select      include instances in a service
  deselect    deselect a group of instances from a service

Flags:
  -h, --help[=false]: help for amberctl
```

### Define and remove services

`amberctl service` is the subcommand to define a service. It needs a
name, and usually you'll supply the address on which the service
should listen. You can specify the protocol for the service -- whether
it should be treated as HTTP or plain TCP -- in the address, or with
another option. (Using HTTP means you get extra, HTTP-specific
metrics.)

It's possible to create a service that has no address. You might do
this if you were going to use it only to control an external load
balancer (like [the edgebal image](../edgebal/README.md)).

There are also options for selecting containers to be instances, as a
shortcut to using a subsequent `amberctl select ...` command.

```
Usage:
  amberctl service <name> [flags]

Flags:
      --address="": in the format <ipaddr>:<port>[/<protocol>], an IP address and port at which the service should be made available on each host; optionally, the protocol to assume.
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
      --port-fixed=0: Use a fixed port, and get the IP address from docker inspect
      --port-mapped=0: Use the host IP address, and the host port mapped to the given container port
  -p, --protocol="": the protocol to assume for connections to the service; either "http" or "tcp". Overrides the protocol given in --address if present.
      --tag="": filter instances for this tag
```

You can remove a service, or all services, with `amberctl rm`:

```
Usage:
  amberctl rm <service>|--all
```

### Select and deselect instances

Once you have a service defined, you can select containers to be
enrolled as instances of the service. Ambergreen will load-balance
connections to the service address amongst the instances.

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
of `amberctl select`; a container will be enrolled if it matches _any_
of the rules.

```
Usage:
  amberctl select <service> <rule> [flags]

Flags:
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
      --port-fixed=0: Use a fixed port, and get the IP address from docker inspect
      --port-mapped=0: Use the host IP address, and the host port mapped to the given container port
      --tag="": filter instances for this tag
```

When you use `amberctl select ...`, you give the rule a name. The name
can be used to remove that rule later. A container may remain enrolled
if it matches another rule.

```
Usage:
  amberctl deselect <service> <rule>
```

### List services and query instances

You can list the currently configured services, and optionally their
selection rules, using `amberctl list`.

```
Usage:
  amberctl list [flags]

Flags:
  -f, --format="": format each service with the go template expression given
      --format-rule="": format each rule with the go template expression given (implies --verbose)
  -v, --verbose[=false]: show the list of selection rules for each service
  ```

You can also query for instances, of a particular service of of any
service, using `amberctl query`.

```
Usage:
  amberctl query [flags]

Flags:
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
  -f, --format="": format each instance according to the go template given
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
  -s, --service="": print only instances in <service>
      --tag="": filter instances for this tag
```
