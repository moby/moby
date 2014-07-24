# Docker: what's next?

This document is a high-level overview of where we want to take Docker next.
It is a curated selection of planned improvements which are either important, difficult, or both.

For a more complete view of planned and requested improvements, see [the Github issues](https://github.com/docker/docker/issues).

To suggest changes to the roadmap, including additions, please write the change as if it were already in effect, and make a pull request.


## Container wiring and service discovery

In its current version, docker doesn’t make it very easy to manipulate multiple containers as a cohesive group (ie. orchestration), and it doesn’t make it seamless for containers to connect to each other as network services (ie. wiring).

To achieve wiring and orchestration with docker today, you need to write glue scripts yourself, or use one several companion tools available, like Orchestra, Shipper, Deis, Pipeworks, etc.

We want the Docker API to support orchestration and wiring natively, so that these tools can cleanly and seamlessly integrate into the Docker user experience, and remain interoperable with each other.


## Better integration with process supervisors

For docker to be fully usable in production, it needs to cleanly integrate with the host machine’s process supervisor of choice. Whether it’s sysV-init, upstart, systemd, runit or supervisord, we want to make sure docker plays nice with your existing system. This will be a major focus of the 0.7 release.


## Plugin API

We want Docker to run everywhere, and to integrate with every devops tool. Those are ambitious goals, and the only way to reach them is with the Docker community. For the community to participate fully, we need an API which allows Docker to be deeply and easily customized.

We are working on a plugin API which will make Docker very, very customization-friendly. We believe it will facilitate the integrations listed above – and many more we didn’t even think about.


## Broader kernel support

Our goal is to make Docker run everywhere, but currently Docker requires Linux version 3.8 or higher with cgroups support. If you’re deploying new machines for the purpose of running Docker, this is a fairly easy requirement to meet. However, if you’re adding Docker to an existing deployment, you may not have the flexibility to update and patch the kernel.

Expanding Docker’s kernel support is a priority. This includes running on older kernel versions, specifically focusing on versions already popular in server deployments such as those used by RHEL and the OpenVZ stack.


## Cross-architecture support

Our goal is to make Docker run everywhere. However currently Docker only runs on x86_64 systems. We plan on expanding architecture support, so that Docker containers can be created and used on more architectures.
