## Container on a routed-mode network, with a published port

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

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

    {{index . "LFilterDocker4"}}

    {{index . "SFilterDocker4"}}

The nat table is:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

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
