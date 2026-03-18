## Container on a user-defined network, with a port published on a loopback address

Adding a network running a container with a port mapped on a loopback address, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 127.0.0.1:8080:80 --name c1 busybox

Most rules are the same as for a port published to a regular [host address][0]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

But, there's an extra rule in raw-PREROUTING to drop remote traffic destined
to the port mapped on the loopback address:

{{index . "chain raw-PREROUTING"}}

[0]: usernet-portmap.md
