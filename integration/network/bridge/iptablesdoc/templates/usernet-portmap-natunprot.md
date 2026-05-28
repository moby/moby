## Container on a nat-unprotected network, with a published port

Running the daemon with the userland proxy disable then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=nat-unprotected \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

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

    {{index . "LNat4"}}

    {{index . "SNat4"}}

</details>

[400]: usernet-portmap.md
[401]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/port_mapping_linux.go#L747
[402]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L261-L266
