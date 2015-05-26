---
page_title: Docker Swarm discovery
page_description: Swarm discovery
page_keywords: docker, swarm, clustering, discovery
---

# Discovery

Docker Swarm comes with multiple Discovery backends.

## Backends

### Hosted Discovery with Docker Hub

First we create a cluster.

```bash
# create a cluster
$ swarm create
6856663cdefdec325839a4b7e1de38e8 # <- this is your unique <cluster_id>
```

Then we create each node and join them to the cluster.

```bash
# on each of your nodes, start the swarm agent
#  <node_ip> doesn't have to be public (eg. 192.168.0.X),
#  as long as the swarm manager can access it.
$ swarm join --advertise=<node_ip:2375> token://<cluster_id>
```

Finally, we start the Swarm manager. This can be on any machine or even
your laptop.

```bash
$ swarm manage -H tcp://<swarm_ip:swarm_port> token://<cluster_id>
```

You can then use regular Docker commands to interact with your swarm.

```bash
docker -H tcp://<swarm_ip:swarm_port> info
docker -H tcp://<swarm_ip:swarm_port> run ...
docker -H tcp://<swarm_ip:swarm_port> ps
docker -H tcp://<swarm_ip:swarm_port> logs ...
...
```

You can also list the nodes in your cluster.

```bash
swarm list token://<cluster_id>
<node_ip:2375>
```

### Using a static file describing the cluster

For each of your nodes, add a line to a file. The node IP address
doesn't need to be public as long the Swarm manager can access it.

```bash
echo <node_ip1:2375> >> /tmp/my_cluster
echo <node_ip2:2375> >> /tmp/my_cluster
echo <node_ip3:2375> >> /tmp/my_cluster
```

Then start the Swarm manager on any machine.

```bash
swarm manage -H tcp://<swarm_ip:swarm_port> file:///tmp/my_cluster
```

And then use the regular Docker commands.

```bash
docker -H tcp://<swarm_ip:swarm_port> info
docker -H tcp://<swarm_ip:swarm_port> run ...
docker -H tcp://<swarm_ip:swarm_port> ps
docker -H tcp://<swarm_ip:swarm_port> logs ...
...
```

You can list the nodes in your cluster.

```bash
$ swarm list file:///tmp/my_cluster
<node_ip1:2375>
<node_ip2:2375>
<node_ip3:2375>
```

### Using etcd

On each of your nodes, start the Swarm agent. The node IP address
doesn't have to be public as long as the swarm manager can access it.

```bash
swarm join --advertise=<node_ip:2375> etcd://<etcd_ip>/<path>
```

Start the manager on any machine or your laptop.

```bash
swarm manage -H tcp://<swarm_ip:swarm_port> etcd://<etcd_ip>/<path>
```

And then use the regular Docker commands.

```bash
docker -H tcp://<swarm_ip:swarm_port> info
docker -H tcp://<swarm_ip:swarm_port> run ...
docker -H tcp://<swarm_ip:swarm_port> ps
docker -H tcp://<swarm_ip:swarm_port> logs ...
...
```

You can list the nodes in your cluster.

```bash
swarm list etcd://<etcd_ip>/<path>
<node_ip:2375>
```

### Using consul

On each of your nodes, start the Swarm agent. The node IP address
doesn't need to be public as long as the Swarm manager can access it.

```bash
swarm join --advertise=<node_ip:2375> consul://<consul_addr>/<path>
```

Start the manager on any machine or your laptop.

```bash
swarm manage -H tcp://<swarm_ip:swarm_port> consul://<consul_addr>/<path>
```

And then use the regular Docker commands.

```bash
docker -H tcp://<swarm_ip:swarm_port> info
docker -H tcp://<swarm_ip:swarm_port> run ...
docker -H tcp://<swarm_ip:swarm_port> ps
docker -H tcp://<swarm_ip:swarm_port> logs ...
...
```

You can list the nodes in your cluster.

```bash
swarm list consul://<consul_addr>/<path>
<node_ip:2375>
```

### Using zookeeper

On each of your nodes, start the Swarm agent. The node IP doesn't have
to be public as long as the swarm manager can access it.

```bash
swarm join --advertise=<node_ip:2375> zk://<zookeeper_addr1>,<zookeeper_addr2>/<path>
```

Start the manager on any machine or your laptop.

```bash
swarm manage -H tcp://<swarm_ip:swarm_port> zk://<zookeeper_addr1>,<zookeeper_addr2>/<path>
```

You can then use the regular Docker commands.

```bash
docker -H tcp://<swarm_ip:swarm_port> info
docker -H tcp://<swarm_ip:swarm_port> run ...
docker -H tcp://<swarm_ip:swarm_port> ps
docker -H tcp://<swarm_ip:swarm_port> logs ...
...
```

You can list the nodes in the cluster.

```bash
swarm list zk://<zookeeper_addr1>,<zookeeper_addr2>/<path>
<node_ip:2375>
```

### Using a static list of IP addresses

Start the manager on any machine or your laptop

```bash
swarm manage -H <swarm_ip:swarm_port> nodes://<node_ip1:2375>,<node_ip2:2375>
```

Or

```bash
swarm manage -H <swarm_ip:swarm_port> <node_ip1:2375>,<node_ip2:2375>
```

Then use the regular Docker commands.

```bash
docker -H <swarm_ip:swarm_port> info
docker -H <swarm_ip:swarm_port> run ...
docker -H <swarm_ip:swarm_port> ps
docker -H <swarm_ip:swarm_port> logs ...
...
```

### Range pattern for IP addresses

The `file` and `nodes` discoveries support a range pattern to specify IP
addresses, i.e., `10.0.0.[10:200]` will be a list of nodes starting from
`10.0.0.10` to `10.0.0.200`.

For example for the `file` discovery method.

```bash
$ echo "10.0.0.[11:100]:2375"   >> /tmp/my_cluster
$ echo "10.0.1.[15:20]:2375"    >> /tmp/my_cluster
$ echo "192.168.1.2:[2:20]375"  >> /tmp/my_cluster
```

Then start the manager.

```bash
swarm manage -H tcp://<swarm_ip:swarm_port> file:///tmp/my_cluster
```

And for the `nodes` discovery method.

```bash
swarm manage -H <swarm_ip:swarm_port> "nodes://10.0.0.[10:200]:2375,10.0.1.[2:250]:2375"
```

## Contributing a new discovery backend

Contributing a new discovery backend is easy, simply implement this
interface:

```go
type Discovery interface {
     Initialize(string, int) error
     Fetch() ([]string, error)
     Watch(WatchCallback)
     Register(string) error
}
```

### Initialize

The parameters are `discovery` location without the scheme and a heartbeat (in seconds).

### Fetch

Returns the list of all the nodes from the discovery.

### Watch

Triggers an update (`Fetch`). This can happen either via a timer (like
`token`) or use backend specific features (like `etcd`).

### Register

Add a new node to the discovery service.

## Docker Swarm documentation index

- [User guide](./index.md)
- [Sheduler strategies](./scheduler/strategy.md)
- [Sheduler filters](./scheduler/filter.md)
- [Swarm API](./API.md)
