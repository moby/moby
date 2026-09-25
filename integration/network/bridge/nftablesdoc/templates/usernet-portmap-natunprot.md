## Container on a nat-unprotected network, with a published port

Running the daemon with the userland proxy disable then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=nat-unprotected \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the [nat mode network][1]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

But ...

The `filter-forward-in` chain has no per-port rule, instead it accepts
packets for any port (needed in case the filter-FORWARD chain's default
policy is "drop"):

{{index . "chain filter-forward-in__bridge1"}}

In chain raw-PREROUTING, there's no "DROP DIRECT ACCESS" rule, so
container can be accessed from outside the host:

{{index . "chain raw-PREROUTING"}}

_The "dnat" and "masquerade" rules are still in-place. And, if the
userland proxy is enabled, it is still started._

[1]: usernet-portmap.md
