# psflip - zero-downtime process flipper

`psflip` is a configurable zero-downtime process flipper. If two instances of your app can run alongside each other, `psflip` gives you zero-downtime app restarts.

## Rationale

Many zero-downtime deployment systems already exists (see "Alternatives" section). Unfortunately, the ones I found have some pre-requisites - they either require a TCP server, a container with network isolation enabled, or must be written in a specific technology.

I needed a zero-downtime deployment system for an existing codebase communicating over Unix sockets with FCGI, where the technology stack varied. I didn't find anything that suited my needs, that's how `psflip` was born.

`psfilp` is built on top of [tableflip](https://github.com/cloudflare/tableflip), and supports the following requirements:

* No old code keeps running after a successful upgrade -- old `psflip` gracefully terminates the child process.
* The new process has a grace period for performing initialization, and must pass a healthcheck before considered healthy.
* When upgrading, crashing during initialization is OK, either on `psflip` side, or on child process side. The old process will never be killed unless the new process is considered healthy.
* Only a single upgrade is ever run in parallel.
* `psflip` can be upgraded with zero-downtime -- replace the `psflip` binary with a new version and follow the upgrade process.
* Child configuration can be updated with zero-downtime -- change the config file and follow the upgrade process.

## How it works

`psflip` supervises an execution of a single `child`, attempting to make its existent as transparent as possible:

* the `child` inherits `psflip`'s environment, and `std{in,out,err}` streams,
* `psflip` proxies any signals to `child` (except the `upgrade` signal -- read more below),
* when the `child` exits, `psflip` exits as well and relays its exit code.

When `psflip` receives an `upgrade` signal (default: `SIGHUP`), it performs the upgrade:

* the old process forks and performs an initialization,
* the new `psflip` re-reads configuration file and spawns a new version of the child,
* the new `psflip` monitors supervises child initialization and validates it passes the defined healthcheck,
* if the new child process crashes or does not initialize in time, new `psflip` terminates the child and exits,
* if the new `psflip` crashes or does not initialize in time, the old `psflip` terminates the new `psflip` and continues to run,
* if new `psfilp` validates the child as healthy, it updates the pidfile and notifies the old `psflip` about successful upgrade,
* upon the notification, the old `psflip` attempts to gracefully terminate its child through a `terminate` signal (default: `SIGTERM`),
* it the child does not shut down in a given `` time, the old `psflip` terminates it through `SIGKILL` and exits.

On Linux, each `psflip` child is spawned with `pdeathsig` enabled, i.e. Linux kernel will automatically terminate the children if `psflip` crashes without cleanup.

## Configuration

See [`examples/`](https://github.com/mwek/psflip/tree/main/examples).

## Integrating with systemd

```ini
[Unit]
Description=Service using psflip

[Service]
ExecStart=psflip -c path/to/configuration.file
ExecReload=/bin/kill -HUP $MAINPID
PIDFile=/path/to/pid.file
```

## Keeping psflip in foreground (e.g. OpenRC, daemontools)

Some supervisors will consider the service unhealthy as soon as the main process exits. This does not play well with `psflip`'s design that terminates the main process on successful upgrade to complete the zero-downtime upgrade of `psflip`.

To mitigate this, the repository comes with a `pidwatch` utility that monitors the lifecycle of a program that owns a pidfile:
```
pidwatch --pidfile /path/to/pid.file -- psflip -c <config file>
```

`pidwatch` will start a `psflip` instance inheriting `std{in,out,err}`, and will proxy all incoming signals to the main `psflip` instance. If possible, it will register itself as a subreaper to capture the new main process as its child; otherwise it will resort to polling the process through `kill -0` every 100 ms.

`pidwatch` will always exit with 0 when the watched process terminates, regardless of the process exit code.

Note, that while `psflip` supports zero-downtime upgrades, this is not the case for `pidwatch`.

## Alternatives for zero-downtime deployments

I personally consider `psflip` a workaround for systems that need a safe zero-downtime deployment, but do not work well with state-of-the-art solutions. Consider using one of the following systems instead:

* [kamal-proxy](https://github.com/basecamp/kamal-proxy) - if your app runs in a container and supports HTTP & Docker network isolation.
* [traefik](https://doc.traefik.io/traefik/) - if your app works with HTTP/TCP application proxy.
* [docker rollout](https://github.com/wowu/docker-rollout) - if your app works with HTTP/TCP application proxy.

During my search for zero-downtime process restart, I also evaluated the following solutions that didn't satisfy my upgrade tenets:

* [start_server](https://metacpan.org/dist/Server-Starter/view/script/start_server) - it assumes the worker as "healthy" after specific amount of time, and then forcefully tears down old worker even if the new one is dead. Once the old worker is terminated, if the new worker is also dead, it attempts to start the worker in a loop, instead of exiting and relying on supervisor restart,
* [huptime](https://github.com/amscanne/huptime) - no healthcheck for child validation,
* [socketmaster](https://github.com/zimbatm/socketmaster) - crashing during initialization brings down the old worker.
