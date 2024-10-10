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

 - In the FORWARD chain, rules 3-6 for the new network have been inserted at
   the top of the chain, pushing the equivalent docker0 rules down to positions
   7-10. (Rules 3-6 were inserted at the top of the chain, then rules 1-2 were
   shuffled back to the top by deleting/recreating, as described above.)
 - In the DOCKER-ISOLATION chains, rules equivalent to the docker0 rules have
   also been inserted for the new bridge.
 - In the DOCKER chain, there is an ACCEPT rule for TCP port 80 packets routed
   to the container's address. This rule is added when the container is created
   (unlike all the other rules so-far, which were created during driver or
   network initialisation). [setPerPortForwarding][1]

[1]: https://github.com/moby/moby/blob/675c2ac2db93e38bb9c5a6615d4155a969535fd9/libnetwork/drivers/bridge/port_mapping_linux.go#L795

And the corresponding nat table:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
