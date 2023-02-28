# Containerized Build and Cross Compilation

The `Dockerfile` supports building and cross compiling docker daemon and extra
tools using [Docker Buildx](https://github.com/docker/buildx) and [BuildKit](https://github.com/moby/buildkit).
A [bake definition](https://docs.docker.com/build/bake/file-definition/) named
`docker-bake.hcl` is in place to ease the build process:

```shell
# build binaries for the current host platform
# output to ./bundles/binary-daemon by default
docker buildx bake
# or
docker buildx bake binary

# build binaries for the current host platform
# output to ./bin
DESTDIR=./bin docker buildx bake

# build dynamically linked binaries
# output to ./bundles/dynbinary-daemon by default
DOCKER_STATIC=0 docker buildx bake
# or
docker buildx bake dynbinary

# build binaries for all supported platforms
docker buildx bake binary-cross

# build binaries for a specific platform
docker buildx bake --set *.platform=linux/arm64
```

## Where to go next

Next, check out [the test framework](test.md).