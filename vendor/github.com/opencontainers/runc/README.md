# runc

[![Go Report Card](https://goreportcard.com/badge/github.com/opencontainers/runc)](https://goreportcard.com/report/github.com/opencontainers/runc)
[![GoDoc](https://godoc.org/github.com/opencontainers/runc?status.svg)](https://godoc.org/github.com/opencontainers/runc)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/588/badge)](https://bestpractices.coreinfrastructure.org/projects/588)
[![gha/validate](https://github.com/opencontainers/runc/workflows/validate/badge.svg)](https://github.com/opencontainers/runc/actions?query=workflow%3Avalidate)
[![gha/ci](https://github.com/opencontainers/runc/workflows/ci/badge.svg)](https://github.com/opencontainers/runc/actions?query=workflow%3Aci)

## Introduction

`runc` is a CLI tool for spawning and running containers according to the OCI specification.

## Releases

`runc` depends on and tracks the [runtime-spec](https://github.com/opencontainers/runtime-spec) repository.
We will try to make sure that `runc` and the OCI specification major versions stay in lockstep.
This means that `runc` 1.0.0 should implement the 1.0 version of the specification.

You can find official releases of `runc` on the [release](https://github.com/opencontainers/runc/releases) page.

## Security

The reporting process and disclosure communications are outlined [here](https://github.com/opencontainers/org/blob/master/SECURITY.md).

### Security Audit
A third party security audit was performed by Cure53, you can see the full report [here](https://github.com/opencontainers/runc/blob/master/docs/Security-Audit.pdf).

## Building

`runc` currently supports the Linux platform with various architecture support.
It must be built with Go version 1.13 or higher.

In order to enable seccomp support you will need to install `libseccomp` on your platform.
> e.g. `libseccomp-devel` for CentOS, or `libseccomp-dev` for Ubuntu

```bash
# create a 'github.com/opencontainers' in your GOPATH/src
cd github.com/opencontainers
git clone https://github.com/opencontainers/runc
cd runc

make
sudo make install
```

You can also use `go get` to install to your `GOPATH`, assuming that you have a `github.com` parent folder already created under `src`:

```bash
go get github.com/opencontainers/runc
cd $GOPATH/src/github.com/opencontainers/runc
make
sudo make install
```

`runc` will be installed to `/usr/local/sbin/runc` on your system.


#### Build Tags

`runc` supports optional build tags for compiling support of various features,
with some of them enabled by default (see `BUILDTAGS` in top-level `Makefile`).

To change build tags from the default, set the `BUILDTAGS` variable for make,
e.g. to disable seccomp:

```bash
make BUILDTAGS=""
```

| Build Tag | Feature                            | Enabled by default | Dependency |
|-----------|------------------------------------|--------------------|------------|
| seccomp   | Syscall filtering                  | yes                | libseccomp |

The following build tags were used earlier, but are now obsoleted:
 - **nokmem** (since runc v1.0.0-rc94 kernel memory settings are ignored)
 - **apparmor** (since runc v1.0.0-rc93 the feature is always enabled)
 - **selinux**  (since runc v1.0.0-rc93 the feature is always enabled)

### Running the test suite

`runc` currently supports running its test suite via Docker.
To run the suite just type `make test`.

```bash
make test
```

There are additional make targets for running the tests outside of a container but this is not recommended as the tests are written with the expectation that they can write and remove anywhere.

You can run a specific test case by setting the `TESTFLAGS` variable.

```bash
# make test TESTFLAGS="-run=SomeTestFunction"
```

You can run a specific integration test by setting the `TESTPATH` variable.

```bash
# make test TESTPATH="/checkpoint.bats"
```

You can run a specific rootless integration test by setting the `ROOTLESS_TESTPATH` variable.

```bash
# make test ROOTLESS_TESTPATH="/checkpoint.bats"
```

You can run a test using your container engine's flags by setting `CONTAINER_ENGINE_BUILD_FLAGS` and `CONTAINER_ENGINE_RUN_FLAGS` variables.

```bash
# make test CONTAINER_ENGINE_BUILD_FLAGS="--build-arg http_proxy=http://yourproxy/" CONTAINER_ENGINE_RUN_FLAGS="-e http_proxy=http://yourproxy/"
```

### Dependencies Management

`runc` uses [Go Modules](https://github.com/golang/go/wiki/Modules) for dependencies management.
Please refer to [Go Modules](https://github.com/golang/go/wiki/Modules) for how to add or update
new dependencies. When updating dependencies, be sure that you are running Go `1.14` or newer.

```
# Update vendored dependencies
make vendor
# Verify all dependencies
make verify-dependencies
```

## Using runc

Please note that runc is a low level tool not designed with an end user
in mind. It is mostly employed by other higher level container software.

Therefore, unless there is some specific use case that prevents the use
of tools like Docker or Podman, it is not recommended to use runc directly.

If you still want to use runc, here's how.

### Creating an OCI Bundle

In order to use runc you must have your container in the format of an OCI bundle.
If you have Docker installed you can use its `export` method to acquire a root filesystem from an existing Docker container.

```bash
# create the top most bundle directory
mkdir /mycontainer
cd /mycontainer

# create the rootfs directory
mkdir rootfs

# export busybox via Docker into the rootfs directory
docker export $(docker create busybox) | tar -C rootfs -xvf -
```

After a root filesystem is populated you just generate a spec in the format of a `config.json` file inside your bundle.
`runc` provides a `spec` command to generate a base template spec that you are then able to edit.
To find features and documentation for fields in the spec please refer to the [specs](https://github.com/opencontainers/runtime-spec) repository.

```bash
runc spec
```

### Running Containers

Assuming you have an OCI bundle from the previous step you can execute the container in two different ways.

The first way is to use the convenience command `run` that will handle creating, starting, and deleting the container after it exits.

```bash
# run as root
cd /mycontainer
runc run mycontainerid
```

If you used the unmodified `runc spec` template this should give you a `sh` session inside the container.

The second way to start a container is using the specs lifecycle operations.
This gives you more power over how the container is created and managed while it is running.
This will also launch the container in the background so you will have to edit
the `config.json` to remove the `terminal` setting for the simple examples
below (see more details about [runc terminal handling](docs/terminals.md)).
Your process field in the `config.json` should look like this below with `"terminal": false` and `"args": ["sleep", "5"]`.


```json
        "process": {
                "terminal": false,
                "user": {
                        "uid": 0,
                        "gid": 0
                },
                "args": [
                        "sleep", "5"
                ],
                "env": [
                        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                        "TERM=xterm"
                ],
                "cwd": "/",
                "capabilities": {
                        "bounding": [
                                "CAP_AUDIT_WRITE",
                                "CAP_KILL",
                                "CAP_NET_BIND_SERVICE"
                        ],
                        "effective": [
                                "CAP_AUDIT_WRITE",
                                "CAP_KILL",
                                "CAP_NET_BIND_SERVICE"
                        ],
                        "inheritable": [
                                "CAP_AUDIT_WRITE",
                                "CAP_KILL",
                                "CAP_NET_BIND_SERVICE"
                        ],
                        "permitted": [
                                "CAP_AUDIT_WRITE",
                                "CAP_KILL",
                                "CAP_NET_BIND_SERVICE"
                        ],
                        "ambient": [
                                "CAP_AUDIT_WRITE",
                                "CAP_KILL",
                                "CAP_NET_BIND_SERVICE"
                        ]
                },
                "rlimits": [
                        {
                                "type": "RLIMIT_NOFILE",
                                "hard": 1024,
                                "soft": 1024
                        }
                ],
                "noNewPrivileges": true
        },
```

Now we can go through the lifecycle operations in your shell.


```bash
# run as root
cd /mycontainer
runc create mycontainerid

# view the container is created and in the "created" state
runc list

# start the process inside the container
runc start mycontainerid

# after 5 seconds view that the container has exited and is now in the stopped state
runc list

# now delete the container
runc delete mycontainerid
```

This allows higher level systems to augment the containers creation logic with setup of various settings after the container is created and/or before it is deleted. For example, the container's network stack is commonly set up after `create` but before `start`.

#### Rootless containers
`runc` has the ability to run containers without root privileges. This is called `rootless`. You need to pass some parameters to `runc` in order to run rootless containers. See below and compare with the previous version.

**Note:** In order to use this feature, "User Namespaces" must be compiled and enabled in your kernel. There are various ways to do this depending on your distribution:
- Confirm `CONFIG_USER_NS=y` is set in your kernel configuration (normally found in `/proc/config.gz`)
- Arch/Debian: `echo 1 > /proc/sys/kernel/unprivileged_userns_clone`
- RHEL/CentOS 7: `echo 28633 > /proc/sys/user/max_user_namespaces`

Run the following commands as an ordinary user:
```bash
# Same as the first example
mkdir ~/mycontainer
cd ~/mycontainer
mkdir rootfs
docker export $(docker create busybox) | tar -C rootfs -xvf -

# The --rootless parameter instructs runc spec to generate a configuration for a rootless container, which will allow you to run the container as a non-root user.
runc spec --rootless

# The --root parameter tells runc where to store the container state. It must be writable by the user.
runc --root /tmp/runc run mycontainerid
```

#### Supervisors

`runc` can be used with process supervisors and init systems to ensure that containers are restarted when they exit.
An example systemd unit file looks something like this.

```systemd
[Unit]
Description=Start My Container

[Service]
Type=forking
ExecStart=/usr/local/sbin/runc run -d --pid-file /run/mycontainerid.pid mycontainerid
ExecStopPost=/usr/local/sbin/runc delete mycontainerid
WorkingDirectory=/mycontainer
PIDFile=/run/mycontainerid.pid

[Install]
WantedBy=multi-user.target
```

## More documentation

* [cgroup v2](./docs/cgroup-v2.md)
* [Checkpoint and restore](./docs/checkpoint-restore.md)
* [systemd cgroup driver](./docs/systemd.md)
* [Terminals and standard IO](./docs/terminals.md)

## License

The code and docs are released under the [Apache 2.0 license](LICENSE).
