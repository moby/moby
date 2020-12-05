[![asciicinema example](https://asciinema.org/a/gPEIEo1NzmDTUu2bEPsUboqmU.png)](https://asciinema.org/a/gPEIEo1NzmDTUu2bEPsUboqmU)

# BuildKit

[![GoDoc](https://godoc.org/github.com/moby/buildkit?status.svg)](https://godoc.org/github.com/moby/buildkit/client/llb)
[![Build Status](https://travis-ci.com/moby/buildkit.svg?branch=master)](https://travis-ci.com/moby/buildkit)
[![Go Report Card](https://goreportcard.com/badge/github.com/moby/buildkit)](https://goreportcard.com/report/github.com/moby/buildkit)
[![codecov](https://codecov.io/gh/moby/buildkit/branch/master/graph/badge.svg)](https://codecov.io/gh/moby/buildkit)

BuildKit is a toolkit for converting source code to build artifacts in an efficient, expressive and repeatable manner.

Key features:

-   Automatic garbage collection
-   Extendable frontend formats
-   Concurrent dependency resolution
-   Efficient instruction caching
-   Build cache import/export
-   Nested build job invocations
-   Distributable workers
-   Multiple output formats
-   Pluggable architecture
-   Execution without root privileges

Read the proposal from https://github.com/moby/moby/issues/32925

Introductory blog post https://blog.mobyproject.org/introducing-buildkit-17e056cc5317

Join `#buildkit` channel on [Docker Community Slack](http://dockr.ly/slack)

:information_source: If you are visiting this repo for the usage of experimental Dockerfile features like `RUN --mount=type=(bind|cache|tmpfs|secret|ssh)`, please refer to [`frontend/dockerfile/docs/experimental.md`](frontend/dockerfile/docs/experimental.md).

:information_source: [BuildKit has been integrated to `docker build` since Docker 18.06 .](https://docs.docker.com/develop/develop-images/build_enhancements/)
You don't need to read this document unless you want to use the full-featured standalone version of BuildKit.

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Used by](#used-by)
- [Quick start](#quick-start)
  - [Starting the `buildkitd` daemon:](#starting-the-buildkitd-daemon)
  - [Exploring LLB](#exploring-llb)
  - [Exploring Dockerfiles](#exploring-dockerfiles)
    - [Building a Dockerfile with `buildctl`](#building-a-dockerfile-with-buildctl)
    - [Building a Dockerfile using external frontend:](#building-a-dockerfile-using-external-frontend)
    - [Building a Dockerfile with experimental features like `RUN --mount=type=(bind|cache|tmpfs|secret|ssh)`](#building-a-dockerfile-with-experimental-features-like-run---mounttypebindcachetmpfssecretssh)
  - [Output](#output)
    - [Image/Registry](#imageregistry)
    - [Local directory](#local-directory)
    - [Docker tarball](#docker-tarball)
    - [OCI tarball](#oci-tarball)
    - [containerd image store](#containerd-image-store)
- [Cache](#cache)
  - [Garbage collection](#garbage-collection)
  - [Export cache](#export-cache)
    - [Inline (push image and cache together)](#inline-push-image-and-cache-together)
    - [Registry (push image and cache separately)](#registry-push-image-and-cache-separately)
    - [Local directory](#local-directory-1)
    - [`--export-cache` options](#--export-cache-options)
    - [`--import-cache` options](#--import-cache-options)
  - [Consistent hashing](#consistent-hashing)
- [Expose BuildKit as a TCP service](#expose-buildkit-as-a-tcp-service)
  - [Load balancing](#load-balancing)
- [Containerizing BuildKit](#containerizing-buildkit)
  - [Podman](#podman)
  - [Kubernetes](#kubernetes)
  - [Daemonless](#daemonless)
- [Opentracing support](#opentracing-support)
- [Running BuildKit without root privileges](#running-buildkit-without-root-privileges)
- [Building multi-platform images](#building-multi-platform-images)
- [Contributing](#contributing)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Used by

BuildKit is used by the following projects:

-   [Moby & Docker](https://github.com/moby/moby/pull/37151) (`DOCKER_BUILDKIT=1 docker build`)
-   [img](https://github.com/genuinetools/img)
-   [OpenFaaS Cloud](https://github.com/openfaas/openfaas-cloud)
-   [container build interface](https://github.com/containerbuilding/cbi)
-   [Tekton Pipelines](https://github.com/tektoncd/catalog) (formerly [Knative Build Templates](https://github.com/knative/build-templates))
-   [the Sanic build tool](https://github.com/distributed-containers-inc/sanic)
-   [vab](https://github.com/stellarproject/vab)
-   [Rio](https://github.com/rancher/rio)
-   [PouchContainer](https://github.com/alibaba/pouch)
-   [Docker buildx](https://github.com/docker/buildx)
-   [Okteto Cloud](https://okteto.com/)
-   [Earthly earthfiles](https://github.com/vladaionescu/earthly)

## Quick start

:information_source: For Kubernetes deployments, see [`examples/kubernetes`](./examples/kubernetes).

BuildKit is composed of the `buildkitd` daemon and the `buildctl` client.
While the `buildctl` client is available for Linux, macOS, and Windows, the `buildkitd` daemon is only available for Linux currently.

The `buildkitd` daemon requires the following components to be installed:
-   [runc](https://github.com/opencontainers/runc) or [crun](https://github.com/containers/crun)
-   [containerd](https://github.com/containerd/containerd) (if you want to use containerd worker)

The latest binaries of BuildKit are available [here](https://github.com/moby/buildkit/releases) for Linux, macOS, and Windows.

[Homebrew package](https://formulae.brew.sh/formula/buildkit) (unofficial) is available for macOS.
```console
$ brew install buildkit
```

To build BuildKit from source, see [`.github/CONTRIBUTING.md`](./.github/CONTRIBUTING.md).

### Starting the `buildkitd` daemon:

You need to run `buildkitd` as the root user on the host.

```bash
$ sudo buildkitd
```

To run `buildkitd` as a non-root user, see [`docs/rootless.md`](docs/rootless.md).

The buildkitd daemon supports two worker backends: OCI (runc) and containerd.

By default, the OCI (runc) worker is used. You can set `--oci-worker=false --containerd-worker=true` to use the containerd worker.

We are open to adding more backends.

The buildkitd daemon listens gRPC API on `/run/buildkit/buildkitd.sock` by default, but you can also use TCP sockets.
See [Expose BuildKit as a TCP service](#expose-buildkit-as-a-tcp-service).

### Exploring LLB

BuildKit builds are based on a binary intermediate format called LLB that is used for defining the dependency graph for processes running part of your build. tl;dr: LLB is to Dockerfile what LLVM IR is to C.

-   Marshaled as Protobuf messages
-   Concurrently executable
-   Efficiently cacheable
-   Vendor-neutral (i.e. non-Dockerfile languages can be easily implemented)

See [`solver/pb/ops.proto`](./solver/pb/ops.proto) for the format definition, and see [`./examples/README.md`](./examples/README.md) for example LLB applications.

Currently, the following high-level languages has been implemented for LLB:

-   Dockerfile (See [Exploring Dockerfiles](#exploring-dockerfiles))
-   [Buildpacks](https://github.com/tonistiigi/buildkit-pack)
-   [Mockerfile](https://matt-rickard.com/building-a-new-dockerfile-frontend/)
-   [Gockerfile](https://github.com/po3rin/gockerfile)
-   [bldr (Pkgfile)](https://github.com/talos-systems/bldr/)
-   [HLB](https://github.com/openllb/hlb)
-   [Earthfile (Earthly)](https://github.com/earthly/earthly)
-   [Cargo Wharf (Rust)](https://github.com/denzp/cargo-wharf)
-   (open a PR to add your own language)

### Exploring Dockerfiles

Frontends are components that run inside BuildKit and convert any build definition to LLB. There is a special frontend called gateway (`gateway.v0`) that allows using any image as a frontend.

During development, Dockerfile frontend (`dockerfile.v0`) is also part of the BuildKit repo. In the future, this will be moved out, and Dockerfiles can be built using an external image.

#### Building a Dockerfile with `buildctl`

```bash
buildctl build \
    --frontend=dockerfile.v0 \
    --local context=. \
    --local dockerfile=.
# or
buildctl build \
    --frontend=dockerfile.v0 \
    --local context=. \
    --local dockerfile=. \
    --opt target=foo \
    --opt build-arg:foo=bar
```

`--local` exposes local source files from client to the builder. `context` and `dockerfile` are the names Dockerfile frontend looks for build context and Dockerfile location.

#### Building a Dockerfile using external frontend:

External versions of the Dockerfile frontend are pushed to https://hub.docker.com/r/docker/dockerfile-upstream and https://hub.docker.com/r/docker/dockerfile and can be used with the gateway frontend. The source for the external frontend is currently located in `./frontend/dockerfile/cmd/dockerfile-frontend` but will move out of this repository in the future ([#163](https://github.com/moby/buildkit/issues/163)). For automatic build from master branch of this repository `docker/dockerfile-upsteam:master` or `docker/dockerfile-upstream:master-experimental` image can be used.

```bash
buildctl build \
    --frontend gateway.v0 \
    --opt source=docker/dockerfile \
    --local context=. \
    --local dockerfile=.
buildctl build \
    --frontend gateway.v0 \
    --opt source=docker/dockerfile \
    --opt context=git://github.com/moby/moby \
    --opt build-arg:APT_MIRROR=cdn-fastly.deb.debian.org
```

#### Building a Dockerfile with experimental features like `RUN --mount=type=(bind|cache|tmpfs|secret|ssh)`

See [`frontend/dockerfile/docs/experimental.md`](frontend/dockerfile/docs/experimental.md).

### Output

By default, the build result and intermediate cache will only remain internally in BuildKit. An output needs to be specified to retrieve the result.

#### Image/Registry

```bash
buildctl build ... --output type=image,name=docker.io/username/image,push=true
```

To export the cache embed with the image and pushing them to registry together, type `registry` is required to import the cache, you should specify `--export-cache type=inline` and `--import-cache type=registry,ref=...`. To export the cache to a local directy, you should specify `--export-cache type=local`.
Details in [Export cache](#export-cache).

```bash
buildctl build ...\
  --output type=image,name=docker.io/username/image,push=true \
  --export-cache type=inline \
  --import-cache type=registry,ref=docker.io/username/image
```

Keys supported by image output:
* `name=[value]`: image name
* `push=true`: push after creating the image
* `push-by-digest=true`: push unnamed image
* `registry.insecure=true`: push to insecure HTTP registry
* `oci-mediatypes=true`: use OCI mediatypes in configuration JSON instead of Docker's
* `unpack=true`: unpack image after creation (for use with containerd)
* `dangling-name-prefix=[value]`: name image with `prefix@<digest>` , used for anonymous images
* `name-canonical=true`: add additional canonical name `name@<digest>`
* `compression=[uncompressed,gzip]`: choose compression type for layer, gzip is default value


If credentials are required, `buildctl` will attempt to read Docker configuration file `$DOCKER_CONFIG/config.json`.
`$DOCKER_CONFIG` defaults to `~/.docker`.

#### Local directory

The local client will copy the files directly to the client. This is useful if BuildKit is being used for building something else than container images.

```bash
buildctl build ... --output type=local,dest=path/to/output-dir
```

To export specific files use multi-stage builds with a scratch stage and copy the needed files into that stage with `COPY --from`.

```dockerfile
...
FROM scratch as testresult

COPY --from=builder /usr/src/app/testresult.xml .
...
```

```bash
buildctl build ... --opt target=testresult --output type=local,dest=path/to/output-dir
```

Tar exporter is similar to local exporter but transfers the files through a tarball.

```bash
buildctl build ... --output type=tar,dest=out.tar
buildctl build ... --output type=tar > out.tar
```

#### Docker tarball

```bash
# exported tarball is also compatible with OCI spec
buildctl build ... --output type=docker,name=myimage | docker load
```

#### OCI tarball

```bash
buildctl build ... --output type=oci,dest=path/to/output.tar
buildctl build ... --output type=oci > output.tar
```
#### containerd image store

The containerd worker needs to be used

```bash
buildctl build ... --output type=image,name=docker.io/username/image
ctr --namespace=buildkit images ls
```

To change the containerd namespace, you need to change `worker.containerd.namespace` in [`/etc/buildkit/buildkitd.toml`](./docs/buildkitd.toml.md).


## Cache

To show local build cache (`/var/lib/buildkit`):

```bash
buildctl du -v
```

To prune local build cache:
```bash
buildctl prune
```

### Garbage collection

See [`./docs/buildkitd.toml.md`](./docs/buildkitd.toml.md).

### Export cache

BuildKit supports the following cache exporters:
* `inline`: embed the cache into the image, and push them to the registry together
* `registry`: push the image and the cache separately
* `local`: export to a local directory

In most case you want to use the `inline` cache exporter.
However, note that the `inline` cache exporter only supports `min` cache mode. 
To enable `max` cache mode, push the image and the cache separately by using `registry` cache exporter.

#### Inline (push image and cache together)

```bash
buildctl build ... \
  --output type=image,name=docker.io/username/image,push=true \
  --export-cache type=inline \
  --import-cache type=registry,ref=docker.io/username/image
```

Note that the inline cache is not imported unless `--import-cache type=registry,ref=...` is provided.

:information_source: Docker-integrated BuildKit (`DOCKER_BUILDKIT=1 docker build`) and `docker buildx`requires 
`--build-arg BUILDKIT_INLINE_CACHE=1` to be specified to enable the `inline` cache exporter.
However, the standalone `buildctl` does NOT require `--opt build-arg:BUILDKIT_INLINE_CACHE=1` and the build-arg is simply ignored.

#### Registry (push image and cache separately)

```bash
buildctl build ... \
  --output type=image,name=localhost:5000/myrepo:image,push=true \
  --export-cache type=registry,ref=localhost:5000/myrepo:buildcache \
  --import-cache type=registry,ref=localhost:5000/myrepo:buildcache \
```

#### Local directory

```bash
buildctl build ... --export-cache type=local,dest=path/to/output-dir
buildctl build ... --import-cache type=local,src=path/to/input-dir
```

The directory layout conforms to OCI Image Spec v1.0.

#### `--export-cache` options
-   `type`: `inline`, `registry`, or `local`
-   `mode=min` (default): only export layers for the resulting image
-   `mode=max`: export all the layers of all intermediate steps. Not supported for `inline` cache exporter.
-   `ref=docker.io/user/image:tag`: reference for `registry` cache exporter
-   `dest=path/to/output-dir`: directory for `local` cache exporter
-   `oci-mediatypes=true|false`: whether to use OCI mediatypes in exported manifests for `local` and `registry` exporter. Since BuildKit `v0.8` defaults to true.

#### `--import-cache` options
-   `type`: `registry` or `local`. Use `registry` to import `inline` cache.
-   `ref=docker.io/user/image:tag`: reference for `registry` cache importer
-   `src=path/to/input-dir`: directory for `local` cache importer
-   `digest=sha256:deadbeef`: digest of the manifest list to import for `local` cache importer.
-   `tag=customtag`: custom tag of image for `local` cache importer.
    Defaults to the digest of "latest" tag in `index.json` is for digest, not for tag

### Consistent hashing

If you have multiple BuildKit daemon instances but you don't want to use registry for sharing cache across the cluster,
consider client-side load balancing using consistent hashing.

See [`./examples/kubernetes/consistenthash`](./examples/kubernetes/consistenthash).

## Expose BuildKit as a TCP service

The `buildkitd` daemon can listen the gRPC API on a TCP socket.

It is highly recommended to create TLS certificates for both the daemon and the client (mTLS).
Enabling TCP without mTLS is dangerous because the executor containers (aka Dockerfile `RUN` containers) can call BuildKit API as well.

```bash
buildkitd \
  --addr tcp://0.0.0.0:1234 \
  --tlscacert /path/to/ca.pem \
  --tlscert /path/to/cert.pem \
  --tlskey /path/to/key.pem
```

```bash
buildctl \
  --addr tcp://example.com:1234 \
  --tlscacert /path/to/ca.pem \
  --tlscert /path/to/clientcert.pem \
  --tlskey /path/to/clientkey.pem \
  build ...
```

### Load balancing

`buildctl build` can be called against randomly load balanced the `buildkitd` daemon.

See also [Consistent hashing](#consistenthashing) for client-side load balancing.

## Containerizing BuildKit

BuildKit can also be used by running the `buildkitd` daemon inside a Docker container and accessing it remotely.

We provide the container images as [`moby/buildkit`](https://hub.docker.com/r/moby/buildkit/tags/):

-   `moby/buildkit:latest`: built from the latest regular [release](https://github.com/moby/buildkit/releases)
-   `moby/buildkit:rootless`: same as `latest` but runs as an unprivileged user, see [`docs/rootless.md`](docs/rootless.md)
-   `moby/buildkit:master`: built from the master branch
-   `moby/buildkit:master-rootless`: same as master but runs as an unprivileged user, see [`docs/rootless.md`](docs/rootless.md)

To run daemon in a container:

```bash
docker run -d --name buildkitd --privileged moby/buildkit:latest
export BUILDKIT_HOST=docker-container://buildkitd
buildctl build --help
```

### Podman
To connect to a BuildKit daemon running in a Podman container, use `podman-container://` instead of `docker-container://` .

```bash
podman run -d --name buildkitd --privileged moby/buildkit:latest
buildctl --addr=podman-container://buildkitd build --frontend dockerfile.v0 --local context=. --local dockerfile=. --output type=oci | podman load foo
```

`sudo` is not required.

### Kubernetes

For Kubernetes deployments, see [`examples/kubernetes`](./examples/kubernetes).

### Daemonless

To run client and an ephemeral daemon in a single container ("daemonless mode"):

```bash
docker run \
    -it \
    --rm \
    --privileged \
    -v /path/to/dir:/tmp/work \
    --entrypoint buildctl-daemonless.sh \
    moby/buildkit:master \
        build \
        --frontend dockerfile.v0 \
        --local context=/tmp/work \
        --local dockerfile=/tmp/work
```

or

```bash
docker run \
    -it \
    --rm \
    --security-opt seccomp=unconfined \
    --security-opt apparmor=unconfined \
    -e BUILDKITD_FLAGS=--oci-worker-no-process-sandbox \
    -v /path/to/dir:/tmp/work \
    --entrypoint buildctl-daemonless.sh \
    moby/buildkit:master-rootless \
        build \
        --frontend \
        dockerfile.v0 \
        --local context=/tmp/work \
        --local dockerfile=/tmp/work
```

## Opentracing support

BuildKit supports opentracing for buildkitd gRPC API and buildctl commands. To capture the trace to [Jaeger](https://github.com/jaegertracing/jaeger), set `JAEGER_TRACE` environment variable to the collection address.

```bash
docker run -d -p6831:6831/udp -p16686:16686 jaegertracing/all-in-one:latest
export JAEGER_TRACE=0.0.0.0:6831
# restart buildkitd and buildctl so they know JAEGER_TRACE
# any buildctl command should be traced to http://127.0.0.1:16686/
```

## Running BuildKit without root privileges

Please refer to [`docs/rootless.md`](docs/rootless.md).

## Building multi-platform images

See [`docker buildx` documentation](https://github.com/docker/buildx#building-multi-platform-images)

## Contributing

Want to contribute to BuildKit? Awesome! You can find information about contributing to this project in the [CONTRIBUTING.md](/.github/CONTRIBUTING.md)

