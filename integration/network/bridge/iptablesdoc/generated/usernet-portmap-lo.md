<!-- This is a generated file; DO NOT EDIT. -->

## Container on a user-defined network, with a port published on a loopback address

Adding a network running a container with a port mapped on a loopback address, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 127.0.0.1:8080:80 --name c1 busybox

The filter and nat tables are identical to [nat mode][0]:

<details>
<summary>filter table</summary>

    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-FORWARD  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     6    --  !bridge1 bridge1  0.0.0.0/0            192.0.2.2            tcp dpt:80
    2        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 DROP       0    --  !bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-BRIDGE (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER     0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-CT (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    2        0     0 ACCEPT     0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    
    Chain DOCKER-FORWARD (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-CT  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-INTERNAL  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    3        0     0 DOCKER-BRIDGE  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    4        0     0 ACCEPT     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    5        0     0 ACCEPT     0    --  bridge1 *       0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-INTERNAL (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER-USER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    

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
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    -A DOCKER-BRIDGE -o docker0 -j DOCKER
    -A DOCKER-BRIDGE -o bridge1 -j DOCKER
    -A DOCKER-CT -o docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A DOCKER-CT -o bridge1 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A DOCKER-FORWARD -j DOCKER-CT
    -A DOCKER-FORWARD -j DOCKER-INTERNAL
    -A DOCKER-FORWARD -j DOCKER-BRIDGE
    -A DOCKER-FORWARD -i docker0 -j ACCEPT
    -A DOCKER-FORWARD -i bridge1 -j ACCEPT
    

</details>

<details>
<summary>nat table</summary>

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
    1        0     0 DNAT       6    --  !bridge1 *       0.0.0.0/0            127.0.0.1            tcp dpt:8080 to:192.0.2.2:80
    

    -P PREROUTING ACCEPT
    -P INPUT ACCEPT
    -P OUTPUT ACCEPT
    -P POSTROUTING ACCEPT
    -N DOCKER
    -A PREROUTING -m addrtype --dst-type LOCAL -j DOCKER
    -A OUTPUT ! -d 127.0.0.0/8 -m addrtype --dst-type LOCAL -j DOCKER
    -A POSTROUTING -s 192.0.2.0/24 ! -o bridge1 -j MASQUERADE
    -A POSTROUTING -s 172.17.0.0/16 ! -o docker0 -j MASQUERADE
    -A DOCKER -d 127.0.0.1/32 ! -i bridge1 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 192.0.2.2:80
    

</details>

    Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  !bridge1 *       0.0.0.0/0            192.0.2.2           
    2        0     0 DROP       6    --  !lo    *       0.0.0.0/0            127.0.0.1            tcp dpt:8080
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    

<details>
<summary>iptables commands</summary>

    -P PREROUTING ACCEPT
    -P OUTPUT ACCEPT
    -A PREROUTING -d 192.0.2.2/32 ! -i bridge1 -j DROP
    -A PREROUTING -d 127.0.0.1/32 ! -i lo -p tcp -m tcp --dport 8080 -j DROP
    

</details>

[filterPortMappedOnLoopback][1] adds an extra rule in the raw-PREROUTING chain to DROP remote traffic destined to the
port mapped on the loopback address.

[0]: usernet-portmap.md
[1]: https://github.com/search?q=repo%3Amoby%2Fmoby%20filterPortMappedOnLoopback&type=code