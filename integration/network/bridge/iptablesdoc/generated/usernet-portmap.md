## Container on a user-defined network, with a published port

Adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is updated as follows:

    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-ISOLATION-STAGE-1  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    3        0     0 ACCEPT     0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    4        0     0 DOCKER     0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    5        0     0 ACCEPT     0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    6        0     0 ACCEPT     0    --  bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    7        0     0 ACCEPT     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    8        0     0 DOCKER     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    9        0     0 ACCEPT     0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    10       0     0 ACCEPT     0    --  docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     6    --  !bridge1 bridge1  0.0.0.0/0            192.0.2.2            tcp dpt:80
    2        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 DROP       0    --  !bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-1 (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-ISOLATION-STAGE-2  0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-ISOLATION-STAGE-2  0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 RETURN     0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-2 (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    2        0     0 DROP       0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 RETURN     0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    
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
    -A FORWARD -j DOCKER-ISOLATION-STAGE-1
    -A FORWARD -o bridge1 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A FORWARD -o bridge1 -j DOCKER
    -A FORWARD -i bridge1 ! -o bridge1 -j ACCEPT
    -A FORWARD -i bridge1 -o bridge1 -j ACCEPT
    -A FORWARD -o docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A FORWARD -o docker0 -j DOCKER
    -A FORWARD -i docker0 ! -o docker0 -j ACCEPT
    -A FORWARD -i docker0 -o docker0 -j ACCEPT
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-1 -i bridge1 ! -o bridge1 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-1 -i docker0 ! -o docker0 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-1 -j RETURN
    -A DOCKER-ISOLATION-STAGE-2 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-2 -o docker0 -j DROP
    -A DOCKER-ISOLATION-STAGE-2 -j RETURN
    -A DOCKER-USER -j RETURN
    

</details>

Note that:

 - In the FORWARD chain, rules 3-6 for the new network have been inserted at
   the top of the chain, pushing the equivalent docker0 rules down to positions
   7-10. (Rules 3-6 were inserted at the top of the chain, then rules 1-2 were
   shuffled back to the top by deleting/recreating, as described above.)
 - In the DOCKER-ISOLATION chains, rules equivalent to the docker0 rules have
   also been inserted for the new bridge.
 - In the DOCKER chain, there is an ACCEPT rule for TCP port 80 packets routed
   to the container's address. This rule is added when the container is created
   (unlike all the other rules so-far, which were created during driver or
   network initialisation). [setPerPortForwarding][1]
   - These per-port rules are inserted at the head of the chain, so that they
     appear before the network's DROP rule [defaultDrop][2] which is always
     appended to the end of the chain. In this case, because `docker0` was
     created before `bridge1`, the `bridge1` rules appear above and below the
     `docker0` DROP rule.

[1]: https://github.com/moby/moby/blob/675c2ac2db93e38bb9c5a6615d4155a969535fd9/libnetwork/drivers/bridge/port_mapping_linux.go#L795
[2]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L252

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
    1        0     0 MASQUERADE  0    --  *      !bridge1  192.0.2.0/24         0.0.0.0/0           
    2        0     0 MASQUERADE  0    --  *      !docker0  172.17.0.0/16        0.0.0.0/0           
    3        0     0 MASQUERADE  6    --  *      *       192.0.2.2            192.0.2.2            tcp dpt:80
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 RETURN     0    --  bridge1 *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 RETURN     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    3        0     0 DNAT       6    --  !bridge1 *       0.0.0.0/0            0.0.0.0/0            tcp dpt:8080 to:192.0.2.2:80
    

<details>
<summary>iptables commands</summary>

    -P PREROUTING ACCEPT
    -P INPUT ACCEPT
    -P OUTPUT ACCEPT
    -P POSTROUTING ACCEPT
    -N DOCKER
    -A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER
    -A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j DOCKER
    -A POSTROUTING -s 192.0.2.0/24 ! -o bridge1 -j MASQUERADE
    -A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE
    -A POSTROUTING -s 192.0.2.2/32 -d 192.0.2.2/32 -p tcp -m tcp --dport 80 -j MASQUERADE
    -A DOCKER -i bridge1 -j RETURN
    -A DOCKER -i docker0 -j RETURN
    -A DOCKER ! -i bridge1 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 192.0.2.2:80
    

</details>
