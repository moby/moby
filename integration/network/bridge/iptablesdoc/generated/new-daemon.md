<!-- This is a generated file; DO NOT EDIT. -->

## iptables for a new Daemon

When the daemon starts, it creates custom chains, and rules for the
default bridge network.

Table `filter`:

    Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain FORWARD (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-USER  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-FORWARD  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    
    Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
    num   pkts bytes target     prot opt in     out     source               destination         
    
    Chain DOCKER (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DROP       0    --  !docker0 docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-BRIDGE (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0           
    
    Chain DOCKER-CT (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 ACCEPT     0    --  *      docker0  0.0.0.0/0            0.0.0.0/0            ctstate RELATED,ESTABLISHED
    
    Chain DOCKER-FORWARD (1 references)
    num   pkts bytes target     prot opt in     out     source               destination         
    1        0     0 DOCKER-CT  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    2        0     0 DOCKER-INTERNAL  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    3        0     0 DOCKER-BRIDGE  0    --  *      *       0.0.0.0/0            0.0.0.0/0           
    4        0     0 ACCEPT     0    --  docker0 *       0.0.0.0/0            0.0.0.0/0           
    
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
    -A DOCKER-BRIDGE -o docker0 -j DOCKER
    -A DOCKER-CT -o docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
    -A DOCKER-FORWARD -j DOCKER-CT
    -A DOCKER-FORWARD -j DOCKER-INTERNAL
    -A DOCKER-FORWARD -j DOCKER-BRIDGE
    -A DOCKER-FORWARD -i docker0 -j ACCEPT
    

</details>

The FORWARD chain's policy shown above is ACCEPT. However:

   - For IPv4, [setupIPv4Forwarding][1] sets the POLICY to DROP if the sysctl
     net.ipv4.ip_forward was not set to '1', and the daemon set it itself when
     an IPv4-enabled bridge network was created.
   - For IPv6, [similar][2], but for sysctls "/proc/sys/net/ipv6/conf/default/forwarding"
     and "/proc/sys/net/ipv6/conf/all/forwarding".

[1]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setupIPv4Forwarding&type=code
[2]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setupIPv6Forwarding&type=code

The FORWARD chain rules, explained in the order they appear in the output above, are:

  1. Unconditional jump to DOCKER-USER.
     This is set up by libnetwork, in [setupUserChain][10].
     Docker won't add rules to the DOCKER-USER chain, it's only for user-defined rules.
     It's (mostly) kept at the top of the by deleting it and re-creating after each
     new network is created, while traffic may be running for other networks.
  2. Unconditional jump to DOCKER-FORWARD.
     This is set up by libnetwork, in [setupIPChains][11].

Once the daemon has initialised, it doesn't touch these rules. Users are free to
append rules to the FORWARD chain, and they'll run after DOCKER's rules (or to
the DOCKER-USER chain, for rules that run before DOCKER's).

The DOCKER-FORWARD chain contains the first stage of Docker's filter rules. Initial
rules are inserted at the top of the table, then not touched. Per-network rules
are appended. The DOCKER-FORWARD chain rules, explained in the order they appear in
the output above, are:

  1. Unconditional jump to DOCKER-CT.
     Created during driver initialisation, in `setupIPChains`.
  2. Unconditional jump to DOCKER-INTERNAL.
     Also created during driver initialisation, in `setupIPChains`.
  3. Unconditional jump to DOCKER-BRIDGE.
     Also created during driver initialisation, in `setupIPChains`.
  4. ACCEPT any packet leaving a network, set up when the network is created, in
     [setupIPTablesInternal][12]. Note that this accepts any packet leaving the
     network that's made it through the DOCKER and isolation chains, whether the
     destination is external or another network.

The DOCKER-CT chain is an early ACCEPT for any RELATED,ESTABLISHED traffic to a
docker bridge. It contains a conntrack ACCEPT rule for each bridge network.

DOCKER-BRIDGE has a rule for each bridge network, to jump to the DOCKER chain.

The DOCKER chain implements per-port/protocol filtering for each container.

[10]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setupUserChain&type=code
[11]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setupIPChains&type=code
[12]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setupNonInternalNetworkRules&type=code

The DOCKER chain has a single DROP rule for the bridge network, to drop any
packets routed to the network that have not originated in the network. Added by
[setDefaultForwardRule][20].
_This means there is no dependency on the filter-FORWARD chain's default policy.
Even if it is ACCEPT, packets will be dropped unless container ports/protocols
are published._

[20]: https://github.com/search?q=repo%3Amoby%2Fmoby%20setDefaultForwardRule&type=code

The DOCKER-INTERNAL chain is for `--internal` networks (bridge networks that
have no external access), it's unused in this example.

Table nat:

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
