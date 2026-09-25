## Container on a routed-mode network, with a published port

Running the daemon with the userland proxy disabled then, as before, adding a network running a container with a mapped port, equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.gateway_mode_ipv4=routed \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

Most rules are the same as the [nat mode network][1]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

But ...

In chain raw-PREROUTING, there's no "DROP DIRECT ACCESS" rule, so
container can be accessed from outside the host:

{{index . "chain raw-PREROUTING"}}

In the `filter-forward-in` chain, there's a rule to accept ICMP:

{{index . "chain filter-forward-in__bridge1"}}

There are no masquerade rules:

{{index . "chain nat-prerouting-and-output"}}
{{index . "chain nat-postrouting-out__bridge1"}}

_And, the userland proxy won't be started for mapped ports._

[1]: usernet-portmap.md
