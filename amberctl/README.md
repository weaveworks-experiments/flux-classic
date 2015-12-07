# amberctl

Command-line interface to set up services and enrol service instances.

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

### Define service

```
Usage:
  amberctl service <name> <IP address> <port> [options] [flags]

Flags:
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
      --fixed=0: Use a fixed port, and get the IP from docker inspect
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
      --mapped=0: Use the host address mapped to the port given
      --protocol="tcp": the protocol to assume for connections to the service; either "http" or "tcp"
      --tag="": filter instances for this tag
```

```
Usage:
  amberctl rm <service>|--all [flags]

Flags:
      --all[=false]: remove all service definitions
```

### Select and deselect instances

```
Usage:
  amberctl select <name> [options] [flags]

Flags:
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
      --fixed=0: Use a fixed port, and get the IP from docker inspect
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
      --mapped=0: Use the host address mapped to the port given
      --protocol="tcp": the protocol to assume for connections to the service; either "http" or "tcp"
      --tag="": filter instances for this tag
```

```
Usage:
  amberctl deselect <service> <group> [flags]
```

### List services and query instances

```
Usage:
  amberctl list [options] [flags]

Flags:
      --format="": format each service with the go template expression given
      --format-instance="": format each instance with the go template expression given (implies verbose)
      --verbose[=false]: show the list of instances for each service
```

```
Usage:
  amberctl query [options] [flags]

Flags:
      --env="": filter instances for these environment variable values, given as comma-delimited key=value pairs
      --format="": format each instance according to the go template given
      --image="": filter instances for this image
      --labels="": filter instances for these labels, given as comma-delimited key=value pairs
      --service="": print only instances in <service>
      --tag="": filter instances for this tag
```
