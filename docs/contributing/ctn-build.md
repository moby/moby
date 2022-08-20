The `Dockerfile` supports building and cross compiling docker daemon and extra
tools using [Docker Buildx](https://github.com/docker/buildx) and [BuildKit](https://github.com/moby/buildkit).
A [bake definition](https://github.com/docker/buildx/blob/master/docs/reference/buildx_bake.md)
named `docker-bake.hcl` is in place to ease the build process:

```shell
# build binaries for the current host platform
# output to ./bundles/binary by default
docker buildx bake

# build binaries for the current host platform
# output to ./bin
DESTDIR=./bin docker buildx bake

# build dynamically linked binaries
# output to ./bundles/dynbinary by default
DOCKER_LINKMODE=dynamic docker buildx bake

# build binaries for all supported platforms
docker buildx bake binary-cross

# build binaries for a specific platform
docker buildx bake --set *.platform=linux/arm64

# build all for the current host platform (binaries + containerd, runc, tini, ...)
# output to ./bundles/all by default
docker buildx bake all

# build all for the current host platform
# output to ./bin
DESTDIR=./bin docker buildx bake all

# build all for all supported platforms
docker buildx bake all-cross
```
