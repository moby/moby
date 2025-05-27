## iptables for a new Daemon

When the daemon starts, it creates custom chains, and rules for the
default bridge network.

Table `filter`:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

</details>

The FORWARD chain's policy shown above is ACCEPT. However:

   - For IPv4, [setupIPForwarding][1] sets the POLICY to DROP if the sysctl
     net.ipv4.ip_forward was not set to '1', and the daemon set it itself when
     an IPv4-enabled bridge network was created.
   - For IPv6, similar, but for sysctls "/proc/sys/net/ipv6/conf/default/forwarding"
     and "/proc/sys/net/ipv6/conf/all/forwarding".

[1]: https://github.com/moby/moby/blob/cff4f20c44a3a7c882ed73934dec6a77246c6323/libnetwork/drivers/bridge/setup_ip_forwarding.go#L44

The FORWARD chain rules, explained in the order they appear in the output above, are:

  1. Unconditional jump to DOCKER-USER.
     This is set up by libnetwork, in [setupUserChain][10].
     Docker won't add rules to the DOCKER-USER chain, it's only for user-defined rules.
     It's (mostly) kept at the top of the by deleting it and re-creating after each
     new network is created, while traffic may be running for other networks.
  2. Unconditional jump to DOCKER-FORWARD.
     This is set up by libnetwork, in [setupUserChain][10].

Once the daemon has initialised, it doesn't touch these rules. Users are free to
append rules to the FORWARD chain, and they'll run after DOCKER's rules (or to
the DOCKER-USER chain, for rules that run before DOCKER's).

The DOCKER-FORWARD chain contains the first stage of Docker's filter rules. Initial
rules are inserted at the top of the table, then not touched. Per-network rules
are appended. The DOCKER-FORWARD chain rules, explained in the order they appear in
the output above, are:

  1. Unconditional jump to DOCKER-CT.
     Created during driver initialisation, in `setupIPChains`.
  2. Unconditional jump to DOCKER-ISOLATION-STAGE-1.
     Also created during driver initialisation, in `setupIPChains`.
  3. Unconditional jump to DOCKER-BRIDGE.
     Also created during driver initialisation, in `setupIPChains`.
  4. ACCEPT any packet leaving a network, set up when the network is created, in
     `setupIPTablesInternal`. Note that this accepts any packet leaving the
     network that's made it through the DOCKER and isolation chains, whether the
     destination is external or another network.

The DOCKER-CT chain is an early ACCEPT for any RELATED,ESTABLISHED traffic to a
docker bridge. It contains a conntrack ACCEPT rule for each bridge network.

DOCKER-BRIDGE has a rule for each bridge network, to jump to the DOCKER chain.

The DOCKER chain implements per-port/protocol filtering for each container.

[10]: https://github.com/moby/moby/blob/e05848c0025b67a16aaafa8cdff95d5e2c064105/libnetwork/firewall_linux.go#L50
[11]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L230-L232
[12]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L227-L229
[13]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L223-L226
[15]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L343

The DOCKER chain has a single DROP rule for the bridge network, to drop any
packets routed to the network that have not originated in the network. Added by
[setDefaultForwardRule][21].
_This means there is no dependency on the filter-FORWARD chain's default policy.
Even if it is ACCEPT, packets will be dropped unless container ports/protocols
are published._

The DOCKER-ISOLATION chains implement inter-network isolation, all (unrelated)
packets are processed by these chains. The rule are inserted at the head of the
chain when a network is created, in [setINC][20].
  - DOCKER-ISOLATION-STAGE-1 jumps to DOCKER-ISOLATION-STAGE-2 for any packet
    routed to a docker network that has not come from that docker network.
  - DOCKER-ISOLATION-STAGE-2 processes all packets leaving a bridge network,
    packets that are destined for any other network are dropped.

[20]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L369
[21]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L252

Table nat:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
