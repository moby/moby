## Container on a user-defined network, with a port published on a specific HostIP

Adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 127.0.0.1:8080:80 --name c1 busybox

The filter and nat tables are the same as with no HostIP specified.

<details>
<summary>Filter table</summary>

    {{index . "LFilter4"}}

    {{index . "SFilter4"}}

</details>

<details>
<summary>NAT table</summary>

    {{index . "LNat4"}}

    {{index . "SNat4"}}

</details>

The raw table is:

    {{index . "LRaw4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SRaw4"}}

</details>

The difference from [port mapping with no HostIP][0] is:

  - An ACCEPT rule is added to the PREROUTING chain to drop packets targeting the
    mapped port and coming from the interface that has the HostIP assigned.
  - And a DROP rule is added too, to drop packets targeting the mapped port but
    didn't pass the previous check.

[0]: usernet-portmap.md