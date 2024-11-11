
# Docker Networking With Libnetwork

This document describes docker networking in bridge and overlay mode delivered via libnetwork. Libnetwork uses iptables extensively to configure NATting and forwarding rules. [https://wiki.archlinux.org/index.php/iptables](https://wiki.archlinux.org/index.php/iptables) provides a good introduction to iptables and its default chains. More details may be found in [http://ipset.netfilter.org/iptables-extensions.man.html](http://ipset.netfilter.org/iptables-extensions.man.html)


# Bridge Mode

![alt_text](images/network_flow_bridge.png "image_tooltip")


The above diagram illustrates the network topology when a container instantiated with network mode set to bridge by docker engine. In this case, the libnetwork does the following



1. Creates a new network namespace container NS  for this container
2. Creates a veth-pair, attaching one end to docker0 bridge on host NS, and move the other end to the new container NS.
3. In the new NS, assigns an IP address from docker0 subnet, sets default route gateway to docker0 IP address.

This completes network setup for container running in bridge mode. Outbound traffic from container flows through routing (Container NS) ? veth-pair ? docker0 bridge (Host NS) ---> docker0 interface (HOST NS) ? routing (HostNS) ? eth0 (HostNS) and out of host. And inbound traffic to container flows through the reverse direction.

Note that the container?s assigned IP (172.17.0.2 in above example) address is on docker0 subnet, and is not visible to externally to host. For this reason, a default masquerading  rule is added to nat iptable?s POSTROUTING chain in host NS  at docker engine initialization time. It states that for request traffic flow that has gone through the routing stage and the srcIP is within docker0 subnet (172.17.0.0/16), the traffic request must be originated from docker containers, therefore its srcIP is replaced with IP of outbound interface determined by routing. In the above diagram eth0?s IP 172.31.2.1 is used by replacement IP. In another word,  masquerade is same as SNAT with replacement srcIP set to outbound interface?s IP.

If the container backends a service and has a listening targetPort in the container NS, it also must also have a corresponding publishedPort in host NS to receive the request and forward it to the container. Two rules are created in host NS for this purpose:



1. In nat iptable, a DOCKER(nat) chain is inserted to PREROUTING chain. And a rule such as ?DNAT tcp any any dport:45999 to conrts? is added to DOCKER chain, it does a DNAT for any traffic arriving at eth0 of host NS with dstIP=172.17.0.2 and dst Port=80, so that the DNATted request become routable to backend container listening on port 80.
2. In filter iptable, a DOCKER(filter) chain is inserted to FORWARD chain. And a rule such as ?ACCEPT tcp any containerIP dport:targetPort? is added. This allows request that is DNATted in 1) to be forwarded container.


# Swarm/Overlay Mode

Libnetwork use completely different set of namespaces, bridges, and iptables to forward container traffic in swarm/overlay mode.


![alt_text](images/network_flow_overlay.png "image_tooltip")


As depicted in the above diagram, when a host joinis a swarm cluster, the docker engine creates following network topology.

Initial Setup ( before any services are created)



1. In host NS, creates a docker_gwbridge bridge, assigning a subnet range to this bridge. In this above diagram 172.18.0.1/16. This subnet is local, and does not leak outside of the host.
2. In host NS, adds masquerading rule in nat iptable (5) PREROUTING chain for any request with srcIP within 172.18.0.0/16 subnet.
3. Creates a new network namespace ingress_sbox NS, creates two veth-pairs, one eth1 connects to docker_gwbridge bridge with fixed IP 172.18.0.2, and other (eth0) connected to ingress NS bridge br0. The eth0 is assigned an IP address, in this example, 10.255.0.2. 
4. In ingress_sbox NS, adds to nat iptable(2)?s PREROUTING chain a rule that snat and redirect service request to ipvs for load-balancing. For instance, 

     ?SNAT  all  --  anywhere 10.255.0.0/16  ipvs to:10.255.0.2?

5. In ingress_sbox NS, adds to nat iptable(2)?s POSTROUTING and OUTPUT chains rules that allows DNS lookup to be redirected to/from docker engine as is required by swarm service discovery.
6. Creates a new network namespace ingress NS, and creates a bridge br0 that has two links, one attaches to eth0 interface on ingress_sbox NS, and other to vxlan interface that in essence makes bridge br0 span across all hosts on the same swarm cluster. Each eth0 interface in ingress_sbox, each container instance of a service, and each service itself are given a unique IP 10.255.xx.xx, and is attached to br0, so that on the same swarm cluster, services, container instances of services, and ingress_sbox?s eth0 are all connected via bridge br0.

When a service is created, say with targetPort=80 and publishedPort=30000. The following are added to the existing network topology. 

Service Setup



1. A container backends the service has its own namespace container NS. And two vether-pairs are created,  eth1 is attached to docker_gwbridge in host NS, is given an IP assigned from docker_gwbridge subnet, in the example,  172.18.0.2, eth0 is attached to br0 in ingress NS, is given an IP of 10.255.0.5 in this example. 
2. In container NS, adds rules to filter iptable (3)?s INPUT and OUTPUT chain that only allows targetPortt =80 traffic.
3. In container NS, adds rule to nat iptables (4)?s PREROUTING chain that changes publishedPort to targetPort. For instance, 

    ?REDIRECT   tcp  --  anywhere  10.255.0.11  tcp dpt:30000 redir ports 80?

4. In container NS, adds rules to nat iptable(4)?s INPUT/OUTPUT chain that allow DNS lookup in this container to be redirected to docker engine.
5. In host NS, in filter iptable (5)?s FORWARD chain, inserts DOCKER-INGRESS chain, and adds a rule to allow service request to port 30000 and its reply. 

    I.e ?ACCEPT  tcp  --  anywhere anywhere tcp dpt:30000? and


    ?ACCEPT tcp  --  anywhere anywhere state RELATED,ESTABLISHED tcp spt:30000?

6. In host NS, in nat iptable(6)?s PREROUTING chain, inserts (different) DOCKER-INGRESS chain, and adds a rule to dnat service request to ingress NS?s eth1?s  IP (172.18.0.2). i.e 

    ?DNAT tcp  --  anywhere anywhere  tcp dpt:30000 to:172.18.0.2:30000?

7. In ingress_sbox NS, in mangle iptable(1)?s PREROUTING chain, adds a rule to mark service request, i.e 

    ?MARK  tcp  --  anywhere anywhere  tcp dpt:30000 MARK set 0x100?

8. In ingress_sbox NS, in nat iptable(2)?s POSTROUTING chain, adds a rule to snat request?s srcIP to eth1?s IP, and forward to ipvs to load-balancing, i.e 

    ?SNAT all  --  0.0.0.0/0  10.255.0.0/16 ipvs to:10.255.0.2?

9. In ingress_sbox, configures ipvs LB policy for marked traffic, i.e 

    FWM  256 rr


      -> 10.255.0.5:0                 Masq    1      0          0         


      -> 10.255.0.7:0                 Masq    1      0          0         


      -> 10.255.0.8:0                 Masq    1      0          0        


    Here each of 10.255.0.x represents IP address of container instance backending the service.



## Service Traffic Flow

This section describes traffic flow of request and reply to/from a service with publishedPort = 30000, targetPort = 80



1. Request arrives at eth0 in host NS, with dstIP=172.31.2.1, dstPort=30000, srcIP=CLIENT_IP, srcPort=CLIENT_PORT. Before routing, It first goes through NAT rule in service setup (6) that dnats request with dstIP=172.18.0.2; It then go through FORWARD rule in service setup (5) during routing that allows request with dstPort=30000 to go through. The routing then forward request to docker_gwbridge, and in turn ...
2. The request arrives at eth1 in ingress_sbox NS with dstIP=172.18.0.2, dstPort=30000, srcIP=CLIENT_IP, srcPort=CLIENT_PORT. Before routing, the request is marked before by mangle iptable rule in service setup (7). After routing, it is snated with eth1?s IP 10.255.0.2, forwarded to ipvs for LB by nat iptable rule in service setup (8). The ipvs policy in setup (9) picks one container instance of the service, in this example 10.255.0.5 and dnats it.
3. The request arrives at br0 in ingress NS with dstIP=10.255.0.5, dstPort=30000, srcIP=10.255.0.2, srcPort=EPHEMERAL_PORT. For simplicity, we assume the container instance 10.255.0,5 is the local host, therefore simply forwards it. Note since br0  spans across all hosts in the cluster via vxlan, with all services instances latching onto it, so whether to pick remote or local container instance,  it does not change the routing policy configuration. 
4. The request arrives at eth0 of container NS with dstIP=10.255.0.5, dstPort=30000, srcIP=10.255.0.x (eth1 IP in ingress_sbox NS), srcPort=EMPHEMERAL_PORT.  Before routing, it?s dstPort is changed to 80 via nat rule in service setup (3), and is allowed to be forwarded to local process by INPUT rule in service setup (2) post routig. The process listening on tcp:80 receives request with dstIP=10.255.0.5,  dstPort=80, srcIP=10.255.0.2, , srcPort=EPHEMERAL_PORT.
5. The process replies, The reply has dstIP=10.255.0.2, dstPort=EPHEMERAL_PORT, srcIp=not_known, srcPort=80. It goes through filter rule in OUTPUT chain in service setup(2), which allows it to pass. It goes through routing that determines outbound interface is eth1, and srcIP=10.255.0.5; and  it ?un-dnats? srcPort=80 to 30000 via nat table rule in service setup (3).
6. The reply arrives at br0 in ingress NS with dstIP=10.255.0.2, dstPort=EPHEMERAL_PORT, srcIP=10.255.0.5, srcPort=30000, which duly forwarded it to ...
7. The eh0 interface in sb_ingress NS. The reply first go through ipvs LB that ?un-dnats? srcIP from 10.255.0.5 to 172.18.0.2; then ?un-snats? via nat rule in service setup (8) dstIP from 10.255.0.2 to CLIENT_IP, dstPort from EPHEMERAL_PORT to CLIENT_PORT.
8. The reply arrives at docker_gwbridge0 interface of host NS with dstIP=CLIENT_IP, dstPort=CLIENT_PORT, srcIP=172.18.0.2, srcPort=30000. The reply ?un-snats? with nat rule in service setup(6) with srcIP changes to 172.31.2.1. And is then forwarded out of eth0 interface, and complete the traffic flow. From external view,  request enters host with dstIP=172.31.2.1, dstPort=30000, srcIP=CLIENT_IP, srcPort=CLIENT_PORT; and reply exits with  dstIP=CLIENT_IP, dstPort=CLIENT_PORT, srcIP=172.31.2.1, srcPort=30000.


## Other Flows

**Northbound traffic originated from a container instance, for example, ping [www.cnn.com](www.cnn.com):**

The traffic flow is exactly the same as in bridge mode, except it is via docker_gwbridge in host NS, and traffic is masqueraded with nat rule in initial setup (2).

**DNS traffic**

DNS lookup traffic is routed to docker engine from container instance for service discovery, filling the blank.


# Other IPTable Chain and Rules

Other iptable chains and rules created and/or managed by docker engine/libnetwork.

**DOCKER-USER**: inserted as the first rule  to FORWARD chain of filter iptable in host NS. So that user can independently managed traffic that may or may not be related docker containers.

**DOCKER-ISOLATION-STAGE-1** / 2: Filling in the blank


<!-- Docs to Markdown version 1.0?17 -->

