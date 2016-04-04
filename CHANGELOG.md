# Changes

## Release v0.2

This second release makes Flux easier to deploy and operate by merging
the agent and balancer into `fluxd`; and, refines its service model
and user interfaces.

*Highlights*:

 * Merge the balancer and agent into one binary `fluxd` (and one container image)
 * Send heartbeat for host from `fluxd`, and report live hosts in `fluxctl info`
 * `fluxd`: withdraw instances that fail connections, and schedule retries with a backoff
 * `flux web`: Show each charts alongside its instance (or group), rather than all together
 * `fluxd`: Survive restarts of etcd, rather than crashing
 * Simplify the way instances are assigned addresses, making it a global setting
 * Make instances that cannot be given an address visible, to help troubleshooting

Between releases, we have improved the documentation, among other
things adding a "Get Started" guide.

## Release v0.1

The initial release provides scripts and container images needed to
run Flux.
