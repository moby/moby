<!-- This is a generated file; DO NOT EDIT. -->

## Container on a nat-unprotected network, with a published port

Running the daemon with the userland proxy disable then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=nat-unprotected \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is:

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
    1        0     0 DROP       all  --  !docker0 docker0  anywhere             anywhere            
    2        0     0 ACCEPT     all  --  !bridge1 bridge1  anywhere             anywhere            
    
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
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER ! -i bridge1 -o bridge1 -j ACCEPT
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

Differences from [nat mode][400]:

  - In the DOCKER chain:
    - Where `nat` mode appended a default-DROP rule for any packets not accepted
      by the per-port/protocol rules, `nat-unprotected` appends a default-ACCEPT
      rule. [setDefaultForwardRule][402]
      - The ACCEPT rule is needed in case the filter-FORWARD chain's default
         policy is DROP.
    - Because the default for this network is ACCEPT, there is no per-port/protocol
      rule to ACCEPT packets for the published port `80/tcp`, [setPerPortIptables][401]
      doesn't set it up.
      - _If the userland proxy is enabled, it is still started._

The nat table is identical to [nat mode][400].

<details>
<summary>nat table</summary>

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

[400]: usernet-portmap.md
[401]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/port_mapping_linux.go#L747
[402]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L261-L266
