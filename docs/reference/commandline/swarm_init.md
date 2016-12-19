---
title: "swarm init"
description: "The swarm init command description and usage"
keywords: "swarm, init"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm init

```markdown
Usage:  docker swarm init [OPTIONS]

Initialize a swarm

Options:
      --advertise-addr string           Advertised address (format: <ip|interface>[:port])
      --autolock                        Enable manager autolocking (requiring an unlock key to start a stopped manager)
      --cert-expiry duration            Validity period for node certificates (ns|us|ms|s|m|h) (default 2160h0m0s)
      --dispatcher-heartbeat duration   Dispatcher heartbeat period (ns|us|ms|s|m|h) (default 5s)
      --external-ca external-ca         Specifications of one or more certificate signing endpoints
      --force-new-cluster               Force create a new cluster from current state
      --help                            Print usage
      --listen-addr node-addr           Listen address (format: <ip|interface>[:port]) (default 0.0.0.0:2377)
      --max-snapshots uint              Number of additional Raft snapshots to retain
      --snapshot-interval uint          Number of log entries between Raft snapshots (default 10000)
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

### `--autolock`

This flag enables automatic locking of managers with an encryption key. The
private keys and data stored by all managers will be protected by the
encryption key printed in the output, and will not be accessible without it.
Thus, it is very important to store this key in order to activate a manager
after it restarts. The key can be passed to `docker swarm unlock` to reactivate
the manager. Autolock can be disabled by running
`docker swarm update --autolock=false`. After disabling it, the encryption key
is no longer required to start the manager, and it will start up on its own
without user intervention.

### `--cert-expiry`

This flag sets the validity period for node certificates.

### `--dispatcher-heartbeat`

This flag sets the frequency with which nodes are told to use as a
period to report their health.

### `--external-ca`

This flag sets up the swarm to use an external CA to issue node certificates. The value takes
the form `protocol=X,url=Y`. The value for `protocol` specifies what protocol should be used
to send signing requests to the external CA. Currently, the only supported value is `cfssl`.
The URL specifies the endpoint where signing requests should be submitted.

### `--force-new-cluster`

This flag forces an existing node that was part of a quorum that was lost to restart as a single node Manager without losing its data.

### `--listen-addr`

The node listens for inbound swarm manager traffic on this address. The default is to listen on
0.0.0.0:2377. It is also possible to specify a network interface to listen on that interface's
address; for example `--listen-addr eth0:2377`.

Specifying a port is optional. If the value is a bare IP address or interface
name, the default port 2377 will be used.

### `--advertise-addr`

This flag specifies the address that will be advertised to other members of the
swarm for API access and overlay networking. If unspecified, Docker will check
if the system has a single IP address, and use that IP address with the
listening port (see `--listen-addr`). If the system has multiple IP addresses,
`--advertise-addr` must be specified so that the correct address is chosen for
inter-manager communication and overlay networking.

It is also possible to specify a network interface to advertise that interface's address;
for example `--advertise-addr eth0:2377`.

Specifying a port is optional. If the value is a bare IP address or interface
name, the default port 2377 will be used.

### `--task-history-limit`

This flag sets up task history retention limit.

### `--max-snapshots`

This flag sets the number of old Raft snapshots to retain in addition to the
current Raft snapshots. By default, no old snapshots are retained. This option
may be used for debugging, or to store old snapshots of the swarm state for
disaster recovery purposes.

### `--snapshot-interval`

This flag specifies how many log entries to allow in between Raft snapshots.
Setting this to a higher number will trigger snapshots less frequently.
Snapshots compact the Raft log and allow for more efficient transfer of the
state to new managers. However, there is a performance cost to taking snapshots
frequently.

## Related information

* [swarm join](swarm_join.md)
* [swarm join-token](swarm_join_token.md)
* [swarm leave](swarm_leave.md)
* [swarm unlock](swarm_unlock.md)
* [swarm unlock-key](swarm_unlock_key.md)
* [swarm update](swarm_update.md)
