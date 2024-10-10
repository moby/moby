## Container on a routed-mode network, with a published port

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is the same as with the userland proxy enabled.

_Note that this means inter-network communication is disabled as-normal so,
although published ports will be directly accessible from a remote host
they are not accessible from containers in neighbouring docker networks
on the same host._

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
