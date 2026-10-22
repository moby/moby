# Moby threat model

This document defines the security scope and untrusted inputs of Moby, the open-source container engine.
Its daemon, `dockerd`, usually runs as root, manages containers through containerd and runc, and exposes the Docker Engine REST API.

Engine API access grants the daemon user's privileges, so unrestricted API clients, host root, Swarm managers, and installed plugins are trusted.
Container processes, image content and build inputs, registries, network traffic, AuthZ-restricted clients, Swarm workers beyond their own tasks, and unprivileged host users are untrusted.
A bug is a vulnerability when one of these can use it to escape or weaken isolation, gain privileges, reach the host or other containers, leak secrets, or seriously disrupt the daemon.
Bugs in containerd, runc, and the kernel go upstream, unless Moby's use of them causes the issue.

## Assets

- Host: filesystem, processes, devices, network, and availability.
- Daemon control plane: the API socket or named pipe, TCP API endpoints, and the containerd socket.
  - Container storage: the data root, container filesystems, volumes, and build cache.
  - Image integrity: content must match its digests.
  - Registry credentials.
- Other containers.
- Swarm cluster state, secrets, and configs.

## Actors

Trusted, because they already have the privileges an attack would give them:

- Host root and the daemon's user.
- Unrestricted API clients, including local users in the `docker` group.
- Swarm managers.
- Installed plugins and drivers, within the privileges granted at install time.

Untrusted:

- Remote hosts on the network, excluding Swarm cluster members.
- AuthZ-restricted API clients.
- Container processes, including rootless containers and Swarm workloads.
- Image and build input authors.
- Registries, including mirrors and insecure registries.
  A registry is trusted only with the credentials given for it, and can send them to the token server it names.
- Swarm workers: trusted with their own tasks, secrets, and configs, but not with the rest of the cluster.
- Local unprivileged host users.

## Trust boundaries and untrusted input

Engine API access grants the daemon user's privileges: root by default, or an unprivileged user in rootless mode.
A bug is not a vulnerability if the attacker needs API access, unless it bypasses the restrictions of an authorization (AuthZ) plugin or gains more privileges than the daemon's user has.

The client sets the container policy: privileges, capabilities, mounts, namespaces, devices, published ports, and security options.
The daemon doesn't limit what clients can ask for, such as bind-mounting `/`, starting privileged containers, or adding capabilities, but it must never grant more than requested.
Isolation weaker than requested, or weaker than the defaults, is in scope even without a working escape.
For example: missing seccomp or AppArmor profiles, capabilities not dropped, masked or read-only paths skipped, a wrong SELinux label, or no-new-privileges dropped.

Image content and build inputs are different.
Layers, image config (labels, environment, `USER`, entrypoint, `VOLUME`, `EXPOSE`, `HEALTHCHECK`, `ONBUILD`), Dockerfiles, and build contexts are attacker-controlled, even when a trusted client passes them in.
They can control the workload inside the container.
Image directives can have their documented effects, but they must not grant privileges or choose host resources the client didn't allow.
For example, `VOLUME` can create volumes and copy image files into them, but must not make the daemon modify host paths outside those volumes.

We treat these as untrusted:

- Processes in containers with default settings (no `--privileged`, default seccomp and AppArmor profiles, default capabilities, no host namespaces or sensitive bind mounts).
  Escapes that affect the host or other containers are in scope, including symlink and TOCTOU races in container filesystem operations: `docker cp`, archive and export endpoints, volume and bind mount setup, and `docker commit`.
- Container and image files the daemon reads, such as `/etc/passwd` and `/etc/group` when resolving `USER`, `--user`, `--group-add`, and `--chown`.
- Images and registries: manifests, configs, layer tarballs, and registry HTTP responses from `docker pull`, and image tarballs for `docker load`.
  Layer extraction that writes outside its target is in scope, and so is copying image content into `VOLUME` volumes.
  Accepting content that doesn't match its digest is in scope.
- Build inputs: Dockerfiles, build contexts, remote Git and URL contexts, `ADD` URLs, and BuildKit frontends.
  Escaping a build container or writing outside the build target is in scope.
- Traffic from containers and remote hosts: the embedded DNS resolver, userland proxy, and iptables and nftables rules for isolation and port publishing.
  Bypasses include reaching unpublished ports from blocked networks, crossing isolated networks, or reaching localhost-bound ports from remote hosts.
  Allowed traffic is not a vulnerability, for example between containers on a bridge with inter-container communication enabled.
- Container logs parsed by logging drivers.
- API clients restricted by an AuthZ plugin.
  AuthZ plugins check HTTP API requests, not gRPC calls.
  They can inspect only JSON request and response bodies.
  For connection upgrades, plugins check the initial HTTP requests, not the later stream.
  Plugins must read JSON the same way the daemon does.
  They cannot reliably filter streamed or partly buffered response bodies.
  Bypassing checks that Moby is supposed to enforce is still in scope.
  See the [AuthZ architecture and limitations](https://docs.docker.com/engine/extend/plugins_authorization/#basic-architecture).
- Swarm nodes without valid join tokens or certificates.
- Swarm workers: reading secrets or configs of tasks not assigned to them, or gaining manager privileges, is in scope.
- Processes inside rootless containers must stay isolated.
  Escaping to the daemon's host user is in scope, even without gaining host root.
- Containers with user-namespace remapping (`--userns-remap`): escaping to an unmapped host user, such as root, is in scope.
- Local unprivileged host users: they must not traverse or read the data root (`/var/lib/docker`), `/run/docker`, or container filesystems, run programs from them, or connect to sockets they have no permission for.

Under [Microsoft's security servicing criteria](https://www.microsoft.com/en-us/msrc/windows-security-servicing-criteria), process-isolated Windows Server containers are not a security boundary against hostile tenants; Hyper-V isolated containers are covered by the virtual machine boundary.
Normal Windows security boundaries still apply to unprivileged container users, including protection against elevation to administrator.
Bugs in Moby's own Windows code, such as path handling in `docker cp` or layer extraction, or permissions on the API named pipe, are in scope.

## Components

Highest priority:

- Container isolation: OCI spec generation, seccomp, AppArmor, capabilities, devices, mounts, and exec.
- Container filesystem operations: archive handling, layers and images, and pulls.
- API server, routing, and request validation.
- Networking.

Lower priority, but still in scope:

- Go client and API types talking to a malicious or compromised daemon.
- Swarm mode.
- Plugin installation and privileges: a plugin must not get more privileges than it declared and the user granted.

Vendored modules from the moby GitHub organization, such as SwarmKit, `go-archive`, `sys`, and `profiles`, are in scope.
Their issues get the priority of the Moby component that uses the affected code.
BuildKit is the exception: report BuildKit issues to BuildKit directly, under its own threat model.
Here, only the Moby-specific BuildKit integration is in scope, such as the executor's AppArmor, user-namespace, and network setup, entitlement handling, and how Moby exposes BuildKit through the Engine API.

Out of scope:

- Other vendored code (for example containerd and `golang.org/x/...` modules).
  Report bugs upstream, unless they're caused by how Moby uses the dependency.
- Bugs in containerd, runc, crun, or the Linux kernel, and the shared-kernel container model itself; stronger isolation needs a VM-based runtime.
  Moby is responsible only for issues caused by its OCI specs or use of these components.
- Attacks that need malicious or compromised plugins.
  Managed and legacy plugins, AuthZ plugins, logging, volume, and network drivers, and NRI plugins are trusted.
- Attacks that need host root, a Swarm manager, or write access to `daemon.json`, the data root, daemon binaries, or other files writable only by the daemon's user.
- Tests and test helpers, development tooling, and contributed utilities (except rootless scripts).
- Deprecated components, except the classic builder.
- Bugs in the Windows kernel or Hyper-V.
  Report these to Microsoft; bugs caused by how Moby uses Windows isolation are still in scope.
- Expected effects of settings documented as insecure, such as `--insecure-registry`, TCP API access without TLS, `--privileged`, `--security-opt seccomp=unconfined`, or Docker socket mounts.
  Bypassing the restrictions that remain is still in scope: disabling seccomp doesn't disable mount isolation, capability restrictions, or AppArmor.

## Vulnerabilities and bugs

A bug is a vulnerability only when one of the untrusted inputs above can use it to:

- escape or weaken isolation requested by the client or set by the defaults,
- gain privileges beyond those it was given,
- access the host or other containers without permission,
- leak credentials or other secrets,
- crash the daemon, or cause hangs or resource exhaustion that seriously disrupt its ability to serve requests.


Everything else is a regular bug.
Documentation describes supported behavior, limits, and security expectations.
Most documentation mistakes are regular bugs, but report incorrect security advice privately if you can show a security impact.
Report regular bugs in public issues as described in `CONTRIBUTING.md`, and report privately when in doubt.

Don't report these as vulnerabilities:

- Findings limited to unit tests, direct calls to internal functions, or unreachable code.
- Denial-of-service reports based only on recovered request-handler panics that don't affect the daemon's ability to serve requests.
- Weaker isolation from privileges the client requested, such as host namespaces or `CAP_SYS_ADMIN`.
- Containers without resource limits using up host CPU, memory, disk, or PIDs; setting limits is up to the client.
  Not applying requested limits is in scope.
- Malicious binaries, vulnerable libraries, or secrets in images; only images that trick the daemon are in scope.
- Sharing between API clients: images, layers, build cache, volumes, and networks are not isolated per client, so an image pulled with one client's credentials is available to all clients.
  Bypassing an AuthZ plugin's restrictions on this is still in scope.
- Wrong cache matches from tampered `--cache-from` sources; cache sources must be trusted by the user.
- Build steps or frontends reading secrets or SSH agent sockets the client passed to the build.
- Panics or errors in the Go client or API types caused by invalid arguments from the calling program.
- Secrets in registry or plugin errors passed through by the daemon.
  Credentials the daemon itself writes to logs or errors are in scope.
- Outdated dependencies without a reachable, demonstrated impact on Moby.

## Reporting

Report privately to security@docker.com as described in `SECURITY.md`, not in public issues.
Show the security impact and a reachable path in a running `dockerd` (API request, CLI command, container process, image, or network traffic).
For issues in the Go client or API types, show a path from a malicious daemon or API endpoint to a program using the Go client.
