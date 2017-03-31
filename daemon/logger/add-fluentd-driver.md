# Proposal: Add logging driver for Fluentd

Now, Docker 1.6 has only syslog driver, except for json-file/none.
This is a proposal to add another Logging driver for [Fluentd](http://www.fluentd.org/).

Fluentd is a log collector middleware, which has plugin system for its input and output to connect many sources and many destinations.
It provides reliable TCP logging communication for Docker containers, and makes users to write these logs where users want. Fluentd is also known as a default logging software for Kubernetes.

Pros:
* Reliable TCP log transferring for container logs
* Fluentd already has many output plugin, so users can send container logs to everywhere([plugin list](http://www.fluentd.org/plugins))

Cons:
* Additional dependencies for fluent-logger-golang and messagepack

I couldn't help writing code for this requested feature :-) https://github.com/tagomoris/docker/tree/logger-driver-fluentd
