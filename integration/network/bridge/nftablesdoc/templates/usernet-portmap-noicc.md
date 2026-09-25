## Container on a user-defined network with inter-container communication disabled, with a published port

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.enable_icc=false \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the network with [icc enabled][0]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

But ...

The `filter-forward-in` chain drops (instead of accepting) packets originating
from the same network:

{{index . "chain filter-forward-in__bridge1"}}

[0]: usernet-portmap.md
