## Container on a user-defined network, with a port published on a loopback address

Adding a network running a container with a port mapped on a loopback address, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 127.0.0.1:8080:80 --name c1 busybox

The filter and nat tables are identical to [nat mode][0]:

<details>
<summary>filter table</summary>

    {{index . "LFilter4"}}

    {{index . "SFilter4"}}

</details>

<details>
<summary>nat table</summary>

    {{index . "LNat4"}}

    {{index . "SNat4"}}

</details>

    {{index . "LRaw4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SRaw4"}}

</details>

[filterPortMappedOnLoopback][1] adds an extra rule in the raw-PREROUTING chain to DROP remote traffic destined to the
port mapped on the loopback address.

[0]: usernet-portmap.md
[1]: https://github.com/search?q=repo%3Amoby%2Fmoby%20filterPortMappedOnLoopback&type=code