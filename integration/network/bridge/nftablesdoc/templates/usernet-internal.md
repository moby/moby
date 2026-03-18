## Containers on user-defined --internal networks

These are the rules for two containers on different `--internal` networks, with and
without inter-container communication (ICC).

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridgeICC \
	  --internal \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridgeICC --name c1 busybox

	docker network create \
	  -o com.docker.network.bridge.name=bridgeNoICC \
	  -o com.docker.network.bridge.enable_icc=true \
	  --internal \
	  --subnet 198.51.100.0/24 --gateway 198.51.100.1 bridge1
	docker run --network bridgeNoICC --name c1 busybox

Most rules are the same as the network with [external access][0]:

<details>
<summary>Full table ...</summary>

{{index . "Ruleset4"}}

</details>

The filter-forward-in chains have rules to drop packets originating outside
the network. And, with ICC disabled, the final verdict is drop rather than
accept:

{{index . "chain filter-forward-in__bridgeICC"}}
{{index . "chain filter-forward-in__bridgeNoICC"}}

The nat-postrouting-out chains have no masquerade rules:

{{index . "chain nat-postrouting-out__bridgeICC"}}
{{index . "chain nat-postrouting-out__bridgeNoICC"}}

[0]: usernet-portmap.md
