# Native Multi-host networking

There is a lot to talk about the native multi-host networking and the `overlay` driver that makes it happen. The technical details are documented under https://github.com/docker/libnetwork/blob/master/docs/overlay.md.
Using the above experimental UI `docker network`, `docker service` and `--publish-service`, the user can exercise the power of multi-host networking.

Since `network` and `service` objects are globally significant, this feature requires distributed states provided by the `libkv` project.
Using `libkv`, the user can plug any of the supported Key-Value store (such as consul, etcd or zookeeper).
User can specify the Key-Value store of choice using the `--kv-store` daemon flag, which takes configuration value of format `PROVIDER:URL`, where
`PROVIDER` is the name of the Key-Value store (such as consul, etcd or zookeeper) and
`URL` is the url to reach the Key-Value store.
Example : `docker daemon --kv-store=consul:localhost:8500`

Send us feedback and comments on [#14083](https://github.com/docker/docker/issues/14083)
or on the usual Google Groups (docker-user, docker-dev) and IRC channels.
