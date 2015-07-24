AppArmor Policies for Docker
----------------------------

This directory contains two files and three policies for Docker:

1. File `docker` for container policies
  * docker-default
  * docker-unconfined
2. File `docker-engine` for the engine policy
  * /usr/bin/docker

These policies are installed by the .deb packages for
Debian-derived distributions, including Ubuntu.

Other packages are free to incorporate these policies
and operating system vendors are encouraged to install
policies with Docker.

Installing Policies
-------------------

Policies should normally be installed through your
operating system packages. However, those developing
Docker, building their own packages, or simply desiring
to install these policies manually may load them using
`apparmor_parser`.

For example:

```
$ sudo apparmor_parser -r contrib/apparmor
```

Refer to your operating system's documentation on
persisting these policies across reboots.


Container Policies
------------------

## Policy definitions

The `docker-default` policy is the default for running
containers. It is a moderately protective policy, while
providing wide application compatability.

The `docker-unconfined` policy is a replacement for a true
'unconfined' policy which executes processes within a
child context. This is the policy used by default
when specifying a privileged container. Users of
privileged containers may still override

The system's standard `unconfined` policy may be used
explicitly by user by specifying the security-opt,
`apparmor:unconfined`. This policy will inherit all
system-wide policies, applying path-based policies
intended for the host system inside of containers.
This was the default for privileged containers
prior to Docker 1.8.


## Applying policies

If AppArmor is enabled on the host, Docker will expect
the `docker-default` and `docker-unconfined` policies
to exist and will apply them automatically when starting
containers.

Users may override the AppArmor profile using the
`security-opt` option (per-container).

For example, the following explicitly specifies the default policy:

```
$ docker run --rm -it --security-opt apparmor:docker-default hello-world
```


Docker Engine Policy
--------------------

A profile exists for the Docker Engine, applied to
both the client and the daemon. This exists to
enforce a principle of least-privilege for the daemon.

This policy is simply called `/usr/bin/docker` and will
apply automatically when loaded into AppArmor.
