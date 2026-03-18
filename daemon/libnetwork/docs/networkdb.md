NetworkDB
=========

There are two databases used in libnetwork:

- A persistent database that stores the network configuration requested by the user. This is typically the SwarmKit managers' raft store.
- A non-persistent peer-to-peer gossip-based database that keeps track of the current runtime state. This is NetworkDB.

NetworkDB is based on the [SWIM][] protocol, which is implemented by the [memberlist][] library.
`memberlist` manages cluster membership (nodes can join and leave), as well as message encryption.
Members of the cluster send each other ping messages from time to time, allowing the cluster to detect when a node has become unavailable.

The information held by each node in NetworkDB is:

- The set of nodes currently in the cluster (plus nodes that have recently left or failed).
- For each peer node, the set of networks to which that node is connected.
- For each of the node's currently-in-use networks, a set of named tables of key/value pairs.
  Note that nodes only keep track of tables for networks to which they belong.

Updates spread through the cluster from node to node, and nodes may have inconsistent views at any given time.
They will eventually converge (quickly, if the network is operating well).
Nodes look up information using their local networkdb instance. Queries are not sent to remote nodes.

NetworkDB does not impose any structure on the tables; they are just maps from `string` keys to `[]byte` values.
Other components in libnetwork use the tables for their own purposes.
For example, there are tables for service discovery and load balancing,
and the [overlay](overlay.md) driver uses NetworkDB to store routing information.
Updates to a network's tables are only shared between nodes that are on that network.

All libnetwork nodes join the gossip cluster.
To do this, they need the IP address and port of at least one other member of the cluster.
In the case of a SwarmKit cluster, for example, each Docker engine will use the IP addresses of the swarm managers as the initial join addresses.
The `Join` method can be used to update these bootstrap IPs if they change while the system is running.

When joining the cluster, the new node will initially synchronise its cluster-wide state (known nodes and networks, but not tables) with at least one other node.
The state will be mostly kept up-to-date by small UDP gossip messages, but each node will also periodically perform a push-pull TCP sync with another random node.
In a push-pull sync, the initiator sends all of its cluster-wide state to the target, and the target then sends all of its own state back in response.

Once part of the gossip cluster, a node will also send a `NodeEventTypeJoin` message, which is a custom message defined by NetworkDB.
This is not actually needed now, but keeping it is useful for backwards compatibility with nodes running previous versions.

While a node is active in the cluster, it can join and leave networks.
When a node wants to join a network, it will send a `NetworkEventTypeJoin` message via gossip to the whole cluster.
It will also perform a bulk-sync of the network-specific state (the tables) with every other node on the network being joined.
This will allow it to get all the network-specific information quickly.
The tables will mostly be kept up-to-date by UDP gossip messages between the nodes on that network, but
each node in the network will also periodically do a full TCP bulk sync of the tables with another random node on the same network.

Note that there are two similar, but separate, gossip-and-periodic-sync mechanisms here:

1. memberlist-provided gossip and push-pull sync of cluster-wide state, involving all nodes in the cluster.
2. networkdb-provided gossip and bulk sync of network tables, for each network, involving just those nodes in that network.

When a node wishes to leave a network, it will send a `NetworkEventTypeLeave` via gossip. It will then delete the network's table data.
When a node hears that another node is leaving a network, it deletes all table entries belonging to the leaving node.
Deleting an entry in this case means marking it for deletion for a while, so that we can detect and ignore any older events that may arrive about it.

When a node wishes to leave the cluster, it will send a `NodeEventTypeLeave` message via gossip.
Nodes receiving this will mark the node as "left".
The leaving node will then send a memberlist leave message too.
If we receive the memberlist leave message without first getting the `NodeEventTypeLeave` one, we mark the node as failed (for a while).
Every node periodically attempts to reconnect to failed nodes, and will do a push-pull sync of cluster-wide state on success.
On success we also send the node a `NodeEventTypeJoin` and then do a bulk sync of network-specific state for all networks that we have in common.

[SWIM]: http://ieeexplore.ieee.org/document/1028914/
[memberlist]: https://github.com/hashicorp/memberlist
