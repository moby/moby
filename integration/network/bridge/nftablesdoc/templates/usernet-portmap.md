## Container on a user-defined network, with a published port

Adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The `ip docker-bridges` table is updated as follows:

{{index . "Ruleset4"}}

The new network has its own set of chains, similar to the chains for docker0 (which
doesn't have any containers), but ...

#### Published port

In chain `filter-forward-in__bridge1`, there's a rule to open the container's
published port:

{{index . "chain filter-forward-in__bridge1"}}

A rule in `raw-PREROUTING` makes sure the container's published port cannot be
accessed from outside the host, because the network has the default gateway
mode "nat":

{{index . "chain raw-PREROUTING"}}

The `nat-prerouting-and-output` chain has a rule to DNAT from host port 8080 to
the container's port 80:

{{index . "chain nat-prerouting-and-output"}}
