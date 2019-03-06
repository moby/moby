## Docker Stacks

This repository contains API definitions and implementations relating
to Docker Stacks, the runtime instantiation of Docker Compose based
applications.

The code is designed to be used standalone, or be vendored into other
projects.


### Standalone Runtime

The Standalone Stacks runtime is a full implementation of the Stacks API and
reconciler for Swarmkit stacks, intended to be ran as a separate container. It
communicates via the Swarmkit API via the local docker socket, and uses a fake
in-memory store for stack objects.

#### Building the standalone runtime

You may build the standalone runtime with

```
make standalone
```

#### Setting up the standalone runtime

The standalone runtime can be ran as a container on a swarmkit manager node:

```
docker run -v /var/run/docker.sock:/var/run/docker.sock -p 8080:2375 dockereng/stack-controller:latest
```

#### Running the End-to-End tests

After building the e2e test image with `make e2e` and starting the standalone runtime (see above) you
can run the e2e tests with something along the following lines:
```
docker run --net host -e DOCKER_HOST=tcp://localhost:8080 dockereng/stack-e2e:latest
```
Additional flags can be passed as command arguments - try `-help` for usage.


## License
docker/stacks is licensed under the Apache License, Version 2.0. See
[LICENSE](https://github.com/docker/stacks/blob/master/LICENSE) for the full
license text.
