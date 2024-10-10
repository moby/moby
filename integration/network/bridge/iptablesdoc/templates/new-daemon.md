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
     net.ipv4.ip_forward was not set to '1', and the daemon set it itself.
   - For IPv6, the policy is always DROP.

[1]: https://github.com/moby/moby/blob/cff4f20c44a3a7c882ed73934dec6a77246c6323/libnetwork/drivers/bridge/setup_ip_forwarding.go#L44

The FORWARD chain rules are numbered in the output above, they are:

  1. Unconditional jump to DOCKER-USER.
     This is set up by libnetwork, in [setupUserChain][10].
     Docker won't add rules to the DOCKER-USER chain, it's only for user-defined rules.
     It's (mostly) kept at the top of the by deleting it and re-creating after each
     new network is created, while traffic may be running for other networks.
  2. Unconditional jump to DOCKER-ISOLATION-STAGE-1.
     Set up during network creation by [setupIPTables][11], which ensures it appears
     after the jump to DOCKER-USER (by deleting it and re-creating, while traffic
     may be running for other networks).
  3. ACCEPT RELATED,ESTABLISHED packets into a specific bridge network.
     Allows responses to outgoing requests, and continuation of incoming requests,
     without needing to process any further rules.
     This rule is also added during network creation, but the code to do it
     is in libnetwork, [ProgramChain][12].
  4. Jump to DOCKER, for any packet destined for a bridge network. Added when
     the network is created, in [ProgramChain][13] ("filterChain" is the DOCKER chain).
     The DOCKER chain implements per-port/protocol filtering for each container.
  5. ACCEPT any packet leaving a network, also set up when the network is created, in
     [setupIPTablesInternal][14].
  6. ACCEPT packets flowing between containers within a network, because by default
     container isolation is disabled. Also set up when the network is created, in
     [setIcc][15].

[10]: https://github.com/moby/moby/blob/e05848c0025b67a16aaafa8cdff95d5e2c064105/libnetwork/firewall_linux.go#L50
[11]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L201
[12]: https://github.com/moby/moby/blob/e05848c0025b67a16aaafa8cdff95d5e2c064105/libnetwork/iptables/iptables.go#L270
[13]: https://github.com/moby/moby/blob/e05848c0025b67a16aaafa8cdff95d5e2c064105/libnetwork/iptables/iptables.go#L251-L255
[14]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L264
[15]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L343

_With ICC enabled 5 and 6 could be combined, to ACCEPT anything from the bridge.
But, when ICC is disabled, rule 6 is DROP, so it would need to be placed before
rule 5. Because the rules are generated in different places, that's a slightly
bigger change than it should be._

The DOCKER chain is empty, because there are no containers with port mappings yet.

The DOCKER-ISOLATION chains implement inter-network isolation, all (unrelated)
packets are processed by these chains. The rule are inserted at the head of the
chain when a network is created, in [setINC][20].
  - DOCKER-ISOLATION-STAGE-1 jumps to DOCKER-ISOLATION-STAGE-2 for any packet
    routed to a docker network that has not come from that docker network.
  - DOCKER-ISOLATION-STAGE-2 processes all packets leaving a bridge network,
    packets that are destined for any other network are dropped.

[20]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L369

Table nat:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
