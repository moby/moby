# Default network options

Docker can apply default options to new networks based on the network driver by configuring `default-network-opts` in `daemon.json`. These defaults are also applied to the automatically created `docker_gwbridge` network used for swarm ingress.

For example, to set the MTU of bridge networks, including `docker_gwbridge`, configure the daemon as follows:

```json
{
  "default-network-opts": {
    "bridge": {
      "com.docker.network.driver.mtu": "1450"
    }
  }
}
```

With this configuration, all new bridge networks, as well as `docker_gwbridge`, are created with an MTU of 1450.
