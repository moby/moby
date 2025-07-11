# Docker Swarm Service Driller(ssd)

ssd is a troubleshooting utility for Docker swarm networks. 

### control-plane and datapath consistency check on a node
ssd checks for the consistency between docker network control-plane (from the docker daemon in-memory state) and kernel data path programming. Currently the tool checks only for the consistency of the Load balancer (implemented using IPVS).

In a three node swarm cluster ssd status for a overlay network `ov2` which has three services running, each replicated to 3 instances.

````bash
vagrant@net-1:~/code/go/src/github.com/docker/docker-e2e/tests$ docker run -v /var/run/docker.sock:/var/run/docker.sock -v /var/run/docker/netns:/var/run/docker/netns --privileged --net=host sanimej/ssd ov2
Verifying LB programming for containers on network ov2
Verifying container /s2.3.ltrdwef0iqf90rqauw3ehcs56...
service s2... OK
service s3... OK
service s1... OK
Verifying container /s3.3.nyhwvdvnocb4wftyhb8dr4fj8...
service s2... OK
service s3... OK
service s1... OK
Verifying container /s1.3.wwx5tuxhnvoz5vrb8ohphby0r...
service s2... OK
service s3... OK
service s1... OK
Verifying LB programming for containers on network ingress
Verifying container Ingress...
service web... OK
````

ssd checks the required iptables programming to direct an incoming packet with the <host ip>:<published port> to the right <backend ip>:<target port>

### control-plane consistency check across nodes in a cluster

Docker networking uses a gossip protocol to synchronize networking state across nodes  in a cluster. ssd's `gossip-consistency` command verifies if the state maintained by all the nodes are consistent.

````bash
In a three node cluster with services running on an overlay network ov2 ssd consistency-checker shows 

vagrant@net-1:~/code/go/src/github.com/docker/docker-e2e/tests$ docker run -v /var/run/docker.sock:/var/run/docker.sock -v /var/run/docker/netns:/var/run/docker/netns --privileged sanimej/ssd ov2 gossip-consistency
Node id: sjfp0ca8f43rvnab6v7f21gq0 gossip hash c57d89094dbb574a37930393278dc282

Node id: bg228r3q9095grj4wxkqs80oe gossip hash c57d89094dbb574a37930393278dc282

Node id: 6jylcraipcv2pxdricqe77j5q gossip hash c57d89094dbb574a37930393278dc282
````

This is hash digest of the control-plane state for the network `ov2` from all the cluster nodes. If the values have a mismatch `docker network inspect --verbose` on the individual nodes can help in identifying what the specific difference is.
