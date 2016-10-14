---
title: "swarm init"
description: "The swarm init command description and usage"
keywords: ["swarm, init"]
---

# swarm init

```markdown
Usage:  docker swarm init [OPTIONS]

Initialize a swarm

Options:
      --advertise-addr value            Advertised address (format: <ip|interface>[:port])
      --cert-expiry duration            Validity period for node certificates (default 2160h0m0s)
      --dispatcher-heartbeat duration   Dispatcher heartbeat period (default 5s)
      --external-ca value               Specifications of one or more certificate signing endpoints
      --force-new-cluster               Force create a new cluster from current state.
      --help                            Print usage
      --listen-addr value               Listen address (format: <ip|interface>[:port])
      --task-history-limit int          Task history retention limit (default 5)
```

Initialize a swarm. The docker engine targeted by this command becomes a manager
in the newly created single-node swarm.


```bash
$ docker swarm init --advertise-addr 192.168.99.121
Swarm initialized: current node (bvz81updecsj6wjz393c09vti) is now a manager.

To add a worker to this swarm, run the following command:

    docker swarm join \
    --token SWMTKN-1-3pu6hszjas19xyp7ghgosyx9k8atbfcr8p2is99znpy26u2lkl-1awxwuwd3z9j1z3puu7rcgdbx \
    172.17.0.2:2377

To add a manager to this swarm, run 'docker swarm join-token manager' and follow the instructions.
```

`docker swarm init` generates two random tokens, a worker token and a manager token. When you join
a new node to the swarm, the node joins as a worker or manager node based upon the token you pass
to [swarm join](swarm_join.md).

After you create the swarm, you can display or rotate the token using
[swarm join-token](swarm_join_token.md).

### `--cert-expiry`

This flag sets the validity period for node certificates.

### `--dispatcher-heartbeat`

This flags sets the frequency with which nodes are told to use as a
period to report their health.

### `--external-ca value`

This flag sets up the swarm to use an external CA to issue node certificates. The value takes
the form `protocol=X,url=Y`. The value for `protocol` specifies what protocol should be used
to send signing requests to the external CA. Currently, the only supported value is `cfssl`.
The URL specifies the endpoint where signing requests should be submitted.

### `--force-new-cluster`

This flag forces an existing node that was part of a quorum that was lost to restart as a single node Manager without losing its data.

### `--listen-addr value`

The node listens for inbound swarm manager traffic on this address. The default is to listen on
0.0.0.0:2377. It is also possible to specify a network interface to listen on that interface's
address; for example `--listen-addr eth0:2377`.

Specifying a port is optional. If the value is a bare IP address or interface
name, the default port 2377 will be used.

### `--advertise-addr value`

This flag specifies the address that will be advertised to other members of the
swarm for API access and overlay networking. If unspecified, Docker will check
if the system has a single IP address, and use that IP address with with the
listening port (see `--listen-addr`). If the system has multiple IP addresses,
`--advertise-addr` must be specified so that the correct address is chosen for
inter-manager communication and overlay networking.

It is also possible to specify a network interface to advertise that interface's address;
for example `--advertise-addr eth0:2377`.

Specifying a port is optional. If the value is a bare IP address or interface
name, the default port 2377 will be used.

### `--task-history-limit`

This flag sets up task history retention limit.

## Related information

* [swarm join](swarm_join.md)
* [swarm leave](swarm_leave.md)
* [swarm update](swarm_update.md)
* [swarm join-token](swarm_join_token.md)
* [node rm](node_rm.md)
