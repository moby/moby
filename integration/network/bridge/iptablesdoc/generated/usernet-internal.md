<!-- This is a generated file; DO NOT EDIT. -->

## Containers on user-defined --internal networks

These are the rules for two containers on different `--internal` networks, with and
without inter-container communication.

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridgeICC \
	  --internal \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridgeICC --name c1 busybox

	docker network create \
	  -o com.docker.network.bridge.name=bridgeNoICC \
	  -o com.docker.network.bridge.enable_icc=true \
	  --internal \
	  --subnet 198.51.100.0/24 --gateway 198.51.100.1 bridge1
	docker run --network bridgeNoICC --name c1 busybox

The filter table is updated as follows:

    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-USER  all  --  any    any     anywhere             anywhere            
    2        0     0 DOCKER-FORWARD  all  --  any    any     anywhere             anywhere            
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       all  --  !docker0 docker0  anywhere             anywhere            
    
    Chain DOCKER-BRIDGE (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     all  --  any    docker0  anywhere             anywhere            
    
    Chain DOCKER-CT (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     all  --  any    docker0  anywhere             anywhere             ctstate RELATED,ESTABLISHED
    
    Chain DOCKER-FORWARD (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-CT  all  --  any    any     anywhere             anywhere            
    2        0     0 DOCKER-INTERNAL  all  --  any    any     anywhere             anywhere            
    3        0     0 DOCKER-BRIDGE  all  --  any    any     anywhere             anywhere            
    4        0     0 ACCEPT     all  --  docker0 any     anywhere             anywhere            
    5        0     0 ACCEPT     all  --  bridgeICC bridgeICC  anywhere             anywhere            
    6        0     0 DROP       all  --  bridgeNoICC bridgeNoICC  anywhere             anywhere            
    
    Chain DOCKER-INTERNAL (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       all  --  any    bridgeNoICC !198.51.100.0/24      anywhere            
    2        0     0 DROP       all  --  bridgeNoICC any     anywhere            !198.51.100.0/24     
    3        0     0 DROP       all  --  any    bridgeICC !192.0.2.0/24         anywhere            
    4        0     0 DROP       all  --  bridgeICC any     anywhere            !192.0.2.0/24        
    
    Chain DOCKER-USER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    

<details>
<summary>iptables commands</summary>

    -P INPUT ACCEPT
    -P FORWARD ACCEPT
    -P OUTPUT ACCEPT
    -N DOCKER
    -N DOCKER-BRIDGE
    -N DOCKER-CT
    -N DOCKER-FORWARD
    -N DOCKER-INTERNAL
    -N DOCKER-USER
    -A FORWARD -j DOCKER-USER
    -A FORWARD -j DOCKER-FORWARD
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER-BRIDGE -o docker0 -j DOCKER
    -A DOCKER-CT -o docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A DOCKER-FORWARD -j DOCKER-CT
    -A DOCKER-FORWARD -j DOCKER-INTERNAL
    -A DOCKER-FORWARD -j DOCKER-BRIDGE
    -A DOCKER-FORWARD -i docker0 -j ACCEPT
    -A DOCKER-FORWARD -i bridgeICC -o bridgeICC -j ACCEPT
    -A DOCKER-FORWARD -i bridgeNoICC -o bridgeNoICC -j DROP
    -A DOCKER-INTERNAL ! -s 198.51.100.0/24 -o bridgeNoICC -j DROP
    -A DOCKER-INTERNAL ! -d 198.51.100.0/24 -i bridgeNoICC -j DROP
    -A DOCKER-INTERNAL ! -s 192.0.2.0/24 -o bridgeICC -j DROP
    -A DOCKER-INTERNAL ! -d 192.0.2.0/24 -i bridgeICC -j DROP
    

</details>

By comparison with the [network with external access][1]:

- In the DOCKER-FORWARD chain, there is no ACCEPT rule for outgoing packets (`-i bridgeINC`).
- There are no rules for this network in the DOCKER chain.
- In DOCKER-INTERNAL:
  - Rule 1 drops any packet routed to the network that does not have a source address in the network's subnet.
  - Rule 2 drops any packet routed out of the network that does not have a dest address in the network's subnet.

The only difference between `bridgeICC` and `bridgeNoICC` is the rule in the DOCKER-FORWARD
chain. To enable ICC, the rule for packets looping through the bridge is ACCEPT. For
no-ICC it's DROP.

[1]: usernet-portmap.md

And the corresponding nat table:

    Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     all  --  any    any     anywhere             anywhere             ADDRTYPE match dst-type LOCAL
    
    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     all  --  any    any     anywhere            !loopback/8           ADDRTYPE match dst-type LOCAL
    
    Chain POSTROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 MASQUERADE  all  --  any    !docker0  172.17.0.0/16        anywhere            
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    

<details>
<summary>iptables commands</summary>

    -P PREROUTING ACCEPT
    -P INPUT ACCEPT
    -P OUTPUT ACCEPT
    -P POSTROUTING ACCEPT
    -N DOCKER
    -A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER
    -A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j DOCKER
    -A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE
    

</details>
