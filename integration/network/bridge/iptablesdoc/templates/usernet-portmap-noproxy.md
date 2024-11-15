## Container on a user-defined network, with a published port, no userland proxy

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

    dockerd --userland-proxy=false
	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is the same as with the userland proxy enabled.

<details>
<summary>Filter table</summary>

    {{index . "LFilter4"}}

    {{index . "SFilter4"}}

</details>

The nat table is:

    {{index . "LNat4"}}
    
<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>

Differences from [running with the proxy][0] are:

  - The jump from the OUTPUT chain to DOCKER happens even for loopback addresses.
    [ProgramChain][1].
  - The "SKIP DNAT" RETURN rule for packets routed to the bridge is omitted from
    the DOCKER chain [setupIPTablesInternal][2].
  - A MASQUERADE rule is added for packets sent from the container to one of its
    own published ports on the host.
  - A MASQUERADE rule for packets from a LOCAL source address is included in
    POSTROUTING [setupIPTablesInternal][3].
  - In the DOCKER chain's DNAT rule, there's no destination bridge [setPerPortNAT][4].

[0]: usernet-portmap.md
[1]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L302
[2]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L293
[3]: https://github.com/moby/moby/blob/333cfa640239153477bf635a8131734d0e9d099d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L302
[4]: https://github.com/moby/moby/blob/675c2ac2db93e38bb9c5a6615d4155a969535fd9/libnetwork/drivers/bridge/port_mapping_linux.go#L772
