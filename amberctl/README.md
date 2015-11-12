# amberctl

Command-line interface to set up services and enrol service instances.

Synopsis:

```
Usage:
  amberctl [command]

Available Commands:
  service     Commands to control services
  enrol       Enrol an instance in a service
  unenrol     Unenrol an instance from a service

Flags:
  -h, --help[=false]: help for amberctl

Usage:
  amberctl service [command]

Available Commands:
  add         Register a new service
  remove      Clear out data for a service or all services
  list        List all registered services

Usage:
  amberctl enrol <service> <instance> <address> <port>

Usage:
  amberctl unenrol <service> <instance>

```
