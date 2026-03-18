<!-- This is a generated file; DO NOT EDIT. -->

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
    1        0     0 DOCKER-USER  all  --  any    any     anywhere             anywhere            
    2        0     0 DOCKER-FORWARD  all  --  any    any     anywhere             anywhere            
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     tcp  --  !bridge1 bridge1  anywhere             192.0.2.2            tcp dpt:http
    2        0     0 DROP       all  --  !docker0 docker0  anywhere             anywhere            
    3        0     0 DROP       all  --  !bridge1 bridge1  anywhere             anywhere            
    
    Chain DOCKER-BRIDGE (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     all  --  any    docker0  anywhere             anywhere            
    2        0     0 DOCKER     all  --  any    bridge1  anywhere             anywhere            
    
    Chain DOCKER-CT (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     all  --  any    docker0  anywhere             anywhere             ctstate RELATED,ESTABLISHED
    2        0     0 ACCEPT     all  --  any    bridge1  anywhere             anywhere             ctstate RELATED,ESTABLISHED
    
    Chain DOCKER-FORWARD (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-CT  all  --  any    any     anywhere             anywhere            
    2        0     0 DOCKER-INTERNAL  all  --  any    any     anywhere             anywhere            
    3        0     0 DOCKER-BRIDGE  all  --  any    any     anywhere             anywhere            
    4        0     0 ACCEPT     all  --  docker0 any     anywhere             anywhere            
    5        0     0 ACCEPT     all  --  bridge1 any     anywhere             anywhere            
    
    Chain DOCKER-INTERNAL (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    
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

Note that:

 - In the DOCKER-FORWARD chain, rule 5 for outgoing traffic from the new network has been
   appended to the end of the chain.
 - The DOCKER-CT and DOCKER-FORWARD chains each have a rule for the new network.
 - In the DOCKER chain, there is an ACCEPT rule for TCP port 80 packets routed
   to the container's address. This rule is added when the container is created
   (unlike all the other rules so-far, which were created during driver or
   network initialisation). [setPerPortForwarding][1]
   - These per-port rules are inserted at the head of the chain, so that they
     appear before the network's DROP rule [setDefaultForwardRule][2] which is
     always appended to the end of the chain. In this case, because `docker0` was
     created before `bridge1`, the `bridge1` rules appear above and below the
     `docker0` DROP rule.

[1]: https://github.com/search?q=repo%3Amoby%2Fmoby+setPerPortForwarding&type=code
[2]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setDefaultForwardRule&type=code

The corresponding nat table:

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
    1        0     0 MASQUERADE  all  --  any    !bridge1  192.0.2.0/24         anywhere            
    2        0     0 MASQUERADE  all  --  any    !docker0  172.17.0.0/16        anywhere            
    
    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DNAT       tcp  --  !bridge1 any     anywhere             anywhere             tcp dpt:http-alt to:192.0.2.2:80
    

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
    -A DOCKER ! -i bridge1 -p tcp -m tcp --dport 8080 -j DNAT --to-destination 192.0.2.2:80
    

</details>

And the raw table:

    Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       all  --  !bridge1 any     anywhere             192.0.2.2           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    

<details>
<summary>iptables commands</summary>

    -P PREROUTING ACCEPT
    -P OUTPUT ACCEPT
    -A PREROUTING -d 192.0.2.2/32 ! -i bridge1 -j DROP
    

</details>

[filterDirectAccess][3] adds a DROP rule to the raw-PREROUTING chain to block direct remote access to the mapped port.

[3]: https://github.com/search?q=repo%3Amoby%2Fmoby%20filterDirectAccess&type=code
