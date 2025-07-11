## nftables for a new Daemon

When the daemon starts, it creates two tables, `ip docker-bridges` and
`ip6 docker-bridges` for IPv4 and IPv6 rules respectively. Each table contains
some base chains and empty verdict maps. Rules for the default bridge network
are then added.

{{index . "Ruleset4"}}

#### filter-FORWARD

Chain `filter-FORWARD` is a base chain, with type `filter` and hook `forward`.
_So, it's equivalent to the iptables built-in chain `FORWARD` in the `filter`
table._ It's initialised with two rules that use the output and input
interface names as keys in verdict maps:

{{index . "chain filter-FORWARD"}}

The verdict maps will be populated with an element per bridge network, each
jumping to a chain containing rules for that bridge. (So, for packets that
aren't going to-or-from a Docker bridge device, no jump rules are found in
the verdict map, and the packets don't need any further processing by this
base chain.)

The filter-FORWARD chain's policy shown above is `accept`. However:

   - For IPv4, the policy is `drop` if the sysctl
     net.ipv4.ip_forward was not set to '1', and the daemon set it itself when
     an IPv4-enabled bridge network was created.
   - For IPv6, similar, but for sysctls "/proc/sys/net/ipv6/conf/default/forwarding"
     and "/proc/sys/net/ipv6/conf/all/forwarding".

#### Per-network filter-FORWARD rules

Chains added for the default bridge network are named after the base chain
hook they're called from, and the network's bridge.

Packets processed by `filter-forward-in__*` will be delivered to the bridge
network if accepted. For docker0, the chain is:

{{index . "chain filter-forward-in__docker0"}}

The rules are:
- conntrack accept for established flows. _Note that accept only applies to the
  base chain, accepted packets may be processed by other base chains registered
  with the same hook._
- accept packets originating within the network, because inter-container
  communication (ICC) is enabled.
- drop any other packets, because there are no containers in the network
  with published ports. _This means there is no dependency on the filter-FORWARD
  chain's default policy. Even if it is ACCEPT, packets will be dropped unless
  container ports/protocols are published._

Packets processed by `filter-forward-out__*` originate from the bridge network:

{{index . "chain filter-forward-out__docker0"}}

The rules in docker0's chain are:
- conntrack accept for established flows.
- an accept rule, containers in this network have access to external networks.

#### nat-POSTROUTING

Like the filter-FORWARD chain, nat-POSTROUTING has a jump to per-network chains
for packets to and from the network.

{{index . "chain nat-POSTROUTING"}}

#### Per-network nat-POSTROUTING rules

In docker0's nat-postrouting chains, there's a single masquerade rule for packets
leaving the network:

{{index . "chain nat-postrouting-in__docker0"}}
{{index . "chain nat-postrouting-out__docker0"}}
