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
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 ACCEPT     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst ctstate RELATED,ESTABLISHED
    3        0     0 DOCKER-ISOLATION-STAGE-1  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    4        0     0 DOCKER     0    --  *      *       0.0.0.0/0            0.0.0.0/0            match-set docker-ext-bridges-v4 dst
    5        0     0 ACCEPT     0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    6        0     0 ACCEPT     0    --  bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    7        0     0 ACCEPT     0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    8        0     0 ACCEPT     0    --  docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     6    --  !bridge1 bridge1  0.0.0.0/0            192.0.2.2            tcp dpt:80
    2        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 ACCEPT     1    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    4        0     0 DROP       0    --  !bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-ISOLATION-STAGE-1 (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     0    --  bridge1 *       0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    2        0     0 RETURN     0    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    3        0     0 DOCKER-ISOLATION-STAGE-2  0    --  docker0 !docker0  0.0.0.0/0            0.0.0.0/0           
    4        0     0 DOCKER-ISOLATION-STAGE-2  0    --  bridge1 !bridge1  0.0.0.0/0            0.0.0.0/0           
    
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
    -A FORWARD -i bridge1 ! -o bridge1 -j ACCEPT
    -A FORWARD -i bridge1 -o bridge1 -j ACCEPT
    -A FORWARD -i docker0 ! -o docker0 -j ACCEPT
    -A FORWARD -i docker0 -o docker0 -j ACCEPT
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER -o bridge1 -p icmp -j ACCEPT
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-1 -i bridge1 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A DOCKER-ISOLATION-STAGE-1 -o bridge1 -j RETURN
    -A DOCKER-ISOLATION-STAGE-1 -i docker0 ! -o docker0 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-1 -i bridge1 ! -o bridge1 -j DOCKER-ISOLATION-STAGE-2
    -A DOCKER-ISOLATION-STAGE-2 -o bridge1 -j DROP
    -A DOCKER-ISOLATION-STAGE-2 -o docker0 -j DROP
    -A DOCKER-USER -j RETURN
    

</details>

Compared to the equivalent [nat mode network][1]:

- In DOCKER-ISOLATION-STAGE-1:
  - Rule 1 accepts outgoing packets related to established connections. This
    is for responses to containers on NAT networks that would not normally
    accept packets from another network, and may have port/protocol filtering
    rules in place that would otherwise drop these responses.
  - Rule 2 skips the jump to DOCKER-ISOLATION-STAGE-2 for any packet routed
    to the routed-mode network. So, it will accept packets from other networks,
    if they make it through the port/protocol filtering rules in the DOCKER
    chain.
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

The ICMP rule, as shown by `iptables -L`, looks alarming until you spot that it's
for `prot 1`:

    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     6    --  !bridge1 bridge1  0.0.0.0/0            192.0.2.2            tcp dpt:80
    2        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    3        0     0 ACCEPT     1    --  *      bridge1  0.0.0.0/0            0.0.0.0/0           
    4        0     0 DROP       0    --  !bridge1 bridge1  0.0.0.0/0            0.0.0.0/0           
    

    -N DOCKER
    -A DOCKER -d 192.0.2.2/32 ! -i bridge1 -o bridge1 -p tcp -m tcp --dport 80 -j ACCEPT
    -A DOCKER ! -i docker0 -o docker0 -j DROP
    -A DOCKER -o bridge1 -p icmp -j ACCEPT
    -A DOCKER ! -i bridge1 -o bridge1 -j DROP
    

The nat table is:

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
    1        0     0 RETURN     0    --  bridge1 *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 RETURN     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    

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
    -A DOCKER -i bridge1 -j RETURN
    -A DOCKER -i docker0 -j RETURN
    

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
