## Container on a user-defined network with inter-container communication disabled, with a published port

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.enable_icc=false \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is:

    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 ACCEPT     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst ctstate RELATED,ESTABLISHED
    3        0     0 DOCKER-ISOLATION-STAGE-1  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    4        0     0 DOCKER     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst
    5        0     0 ACCEPT     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    6        0     0 DROP       0    --  bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    7        0     0 ACCEPT     0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     6    --  !bridge1 bridge1  0.0.0.0/0            192.0.2.2            tcp dpt:80
    2        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 DROP       0    --  !bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-1 (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-ISOLATION-STAGE-2  0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-ISOLATION-STAGE-2  0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-2 (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    2        0     0 DROP       0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    
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
    -A FORWARD -i bridge1 -o bridge1 -j DROP
    -A FORWARD -i bridge1 ! -o bridge1 -j ACCEPT
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-1 -i docker0 ! -o docker0 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-1 -i bridge1 ! -o bridge1 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-2 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-2 -o docker0 -j DROP
    -A DOCKER-USER -j RETURN
    

</details>

By comparison with [ICC=true][1]:

  - Rules 6 and 7 replace the accept rule for outgoing packets.
    - Rule 6, added by `setIcc`, drops any packet sent from the internal network to itself.
    - Rule 7, added by `setupIPTablesInternal` accepts any other outgoing packet.

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
    1        0     0 MASQUERADE  0    --  *      !bridge1  192.0.2.0/24         0.0.0.0/0           
    2        0     0 MASQUERADE  0    --  *      !docker0  172.17.0.0/16        0.0.0.0/0           
    
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
    -A DOCKER -i bridge1 -j RETURN
    -A DOCKER -i docker0 -j RETURN
    -A DOCKER ! -i bridge1 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 192.0.2.2:80
    

</details>
