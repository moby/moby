## Container on a user-defined network, with a published port

Adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is updated as follows:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

</details>

Note that:

 - In the DOCKER-FORWARD chain, rule 5 for outgoing traffic from the new network has been
   appended to the end of the chain.
 - The DOCKER-CT and DOCKER-FORWARD chains each have a rule for the new network.
 - In the DOCKER-ISOLATION chains, rules equivalent to the docker0 rules have
   also been inserted for the new bridge.
 - In the DOCKER chain, there is an ACCEPT rule for TCP port 80 packets routed
   to the container's address. This rule is added when the container is created
   (unlike all the other rules so-far, which were created during driver or
   network initialisation). [setPerPortForwarding][1]
   - These per-port rules are inserted at the head of the chain, so that they
     appear before the network's DROP rule [setDefaultForwardRule][2] which is
     always appended to the end of the chain. In this case, because `docker0` was
     created before `bridge1`, the `bridge1` rules appear above and below the
     `docker0` DROP rule.

[1]: https://github.com/moby/moby/blob/675c2ac2db93e38bb9c5a6615d4155a969535fd9/libnetwork/drivers/bridge/port_mapping_linux.go#L795
[2]: https://github.com/robmry/moby/blob/52c89d467fc5326149e4bbb8903d23589b66ff0d/libnetwork/drivers/bridge/setup_ip_tables_linux.go#L252

The corresponding nat table:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>

And the raw table:

    {{index . "LRaw4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SRaw4"}}

</details>

[filterDirectAccess][3] adds a DROP rule to the raw-PREROUTING chain to block direct remote access to the mapped port.

[3]: https://github.com/search?q=repo%3Amoby%2Fmoby%20filterDirectAccess&type=code
