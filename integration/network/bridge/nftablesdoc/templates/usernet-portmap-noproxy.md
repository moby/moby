## Container on a user-defined network, with a published port, no userland proxy

Running the daemon with the userland proxy disabled then, as before, adding a
network running a container with a mapped port. Equivalent to:

    dockerd --userland-proxy=false
	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the network with the [proxy enabled][0]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

But ...

The jump from nat-OUTPUT chain to nat-prerouting-and-output happens for loopback
addresses, to DNAT packets from one network sent to a port published to the
loopback address by a container in another network - there's no proxy to catch
them ...

{{index . "chain nat-OUTPUT"}}

The rule to DNAT from the host port to the container's port is not restricted
to packets from the network with the published port. Again, there's no proxy
to catch them:

{{index . "chain nat-prerouting-and-output"}}

The `nat-postrouting-in` chains have masquerade rules for packets sent from 
local addresses. And, the chain for bridge1 (which has a container with a published
port) has a masquerade rule for packets sent from the container to its own published
port on the host:

{{index . "chain nat-postrouting-in__docker0"}}
{{index . "chain nat-postrouting-in__bridge1"}}

[0]: usernet-portmap.md
