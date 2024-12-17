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
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 ACCEPT     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst ctstate RELATED,ESTABLISHED
    3        0     0 DOCKER-ISOLATION-STAGE-1  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    4        0     0 DOCKER     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst
    5        0     0 ACCEPT     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    6        0     0 ACCEPT     0    --  bridgeICC bridgeICC  0.0.0.0/0            0.0.0.0/0           
    7        0     0 DROP       0    --  bridgeNoICC bridgeNoICC  0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-1 (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  *      bridgeNoICC !198.51.100.0/24      0.0.0.0/0           
    2        0     0 DROP       0    --  bridgeNoICC *       0.0.0.0/0           !198.51.100.0/24     
    3        0     0 DROP       0    --  *      bridgeICC !192.0.2.0/24         0.0.0.0/0           
    4        0     0 DROP       0    --  bridgeICC *       0.0.0.0/0           !192.0.2.0/24        
    5        0     0 DOCKER-ISOLATION-STAGE-2  0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-2 (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-USER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 RETURN     0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    

<details>
<summary>iptables commands</summary>

    -P INPUT ACCEPT
    -P FORWARD ACCEPT
    -P OUTPUT ACCEPT
    -N DOCKER
    -N DOCKER-ISOLATION-STAGE-1
    -N DOCKER-ISOLATION-STAGE-2
    -N DOCKER-USER
    -A FORWARD -j DOCKER-USER
    -A FORWARD -m set --match-set docker-ext-bridges-v4 dst -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A FORWARD -j DOCKER-ISOLATION-STAGE-1
    -A FORWARD -m set --match-set docker-ext-bridges-v4 dst -j DOCKER
    -A FORWARD -i docker0 -j ACCEPT
    -A FORWARD -i bridgeICC -o bridgeICC -j ACCEPT
    -A FORWARD -i bridgeNoICC -o bridgeNoICC -j DROP
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER-ISOLATION-STAGE-1 ! -s 198.51.100.0/24 -o bridgeNoICC -j DROP
    -A DOCKER-ISOLATION-STAGE-1 ! -d 198.51.100.0/24 -i bridgeNoICC -j DROP
    -A DOCKER-ISOLATION-STAGE-1 ! -s 192.0.2.0/24 -o bridgeICC -j DROP
    -A DOCKER-ISOLATION-STAGE-1 ! -d 192.0.2.0/24 -i bridgeICC -j DROP
    -A DOCKER-ISOLATION-STAGE-1 -i docker0 ! -o docker0 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-2 -o docker0 -j DROP
    -A DOCKER-USER -j RETURN
    

</details>

By comparison with the [network with external access][1]:

- In the FORWARD chain, there is no ACCEPT rule for outgoing packets (`-i bridgeINC`).
- There are no rules for this network in the DOCKER chain.
- In DOCKER-ISOLATION-STAGE-1:
  - Rule 1 drops any packet routed to the network that does not have a source address in the network's subnet.
  - Rule 2 drops any packet routed out of the network that does not have a dest address in the network's subnet.
  - There is no jump to DOCKER-ISOLATION-STAGE-2.
- DOCKER-ISOLATION-STAGE-2 is unused.

The only difference between `bridgeICC` and `bridgeNoICC` is the rule in the FORWARD
chain. To enable ICC, the rule for packets looping through the bridge is ACCEPT. For
no-ICC it's DROP.

[1]: usernet-portmap.md

And the corresponding nat table:

    Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     0    --  *      *       0.0.0.0/0            0.0.0.0/0            ADDRTYPE match dst-type LOCAL
    
    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     0    --  *      *       0.0.0.0/0           !127.0.0.0/8          ADDRTYPE match dst-type LOCAL
    
    Chain POSTROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 MASQUERADE  0    --  *      !docker0  172.17.0.0/16        0.0.0.0/0           
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 RETURN     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    

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
    -A DOCKER -i docker0 -j RETURN
    

</details>
