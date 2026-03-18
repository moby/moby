<!-- This is a generated file; DO NOT EDIT. -->

## Container on a routed-mode network, with a published port

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
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
    1        0     0 ACCEPT     tcp  --  !bridge1 bridge1  anywhere             192.0.2.2            tcp dpt:http
    2        0     0 DROP       all  --  !docker0 docker0  anywhere             anywhere            
    3        0     0 ACCEPT     icmp --  any    bridge1  anywhere             anywhere            
    4        0     0 DROP       all  --  !bridge1 bridge1  anywhere             anywhere            
    
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
    -A DOCKER -o bridge1 -p icmp -j ACCEPT
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

Compared to the equivalent [nat mode network][1]:

- In the DOCKER chain:
  - A rule is added by [setICMP][5] to allow ICMP.
    *ALL* ICMP message types are allowed.
    The equivalent IPv6 rule uses `-p icmpv6` rather than `-p icmp`. 
    - Because the ICMP rule (rule 3) is per-network, it is appended to the chain along
      with the default-DROP rule (rule 4). So, it is likely to be separated from
      per-port/protocol ACCEPT rules for published ports on the same network. But it
      will always appear before the default-DROP.

_[RFC 4890 section 4.3][6] makes recommendations for filtering ICMPv6. These
have been considered, but the host firewall is not a network boundary in the
sense used by the RFC. So, Node Information and Router Renumbering messages are
not discarded, and experimental/unused types are allowed because they may be
needed._

    Chain DOCKER (2 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     tcp  --  !bridge1 bridge1  anywhere             192.0.2.2            tcp dpt:http
    2        0     0 DROP       all  --  !docker0 docker0  anywhere             anywhere            
    3        0     0 ACCEPT     icmp --  any    bridge1  anywhere             anywhere            
    4        0     0 DROP       all  --  !bridge1 bridge1  anywhere             anywhere            
    

    -N DOCKER
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER -o bridge1 -p icmp -j ACCEPT
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    

The nat table is:

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

Differences from [nat mode][1]:

  - In the POSTROUTING chain:
    - No MASQUERADE rule for traffic from the bridge network to elsewhere. [setupIPTablesInternal][2]
    - No MASQUERADE rule for traffic from the bridge network to itself on published port 80 (port
      mapping is skipped). [attemptBindHostPorts][3]
  - In the DOCKER chain:
    - No early return ("skip DNAT") for traffic from the bridge network. [setupIPTablesInternal][4]
    - No DNAT rule for the published port (port mapping is skipped). [attemptBindHostPorts][3]

_And, the userland proxy won't be started for mapped ports._

[1]: usernet-portmap.md
[2]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L294
[3]: https://github.com/moby/moby/blob/675c2ac2db93e38bb9c5a6615d4155a969535fd9/libnetwork/drivers/bridge/port_mapping_linux.go#L477-L479
[4]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L290
[5]: https://github.com/robmry/moby/blob/d456d79cfc12cd7c801eebce0550b645c5343ca6/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L390-L395
[6]: https://www.rfc-editor.org/rfc/rfc4890#section-4.3
