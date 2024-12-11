## Containers on user-defined --internal networks

These are the rules for two containers on different `--internal` networks, with and
without inter-container communication.

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

The filter table is updated as follows:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

</details>

By comparison with the [network with external access][1]:

- In the FORWARD chain, there is no ACCEPT rule for outgoing packets (`-i bridgeINC`).
- There are no rules for this network in the DOCKER chain.
- In DOCKER-ISOLATION-STAGE-1:
  - Rule 1 drops any packet routed to the network that does not have a source address in the network's subnet.
  - Rule 2 drops any packet routed out of the network that does not have a dest address in the network's subnet.
  - There is no jump to DOCKER-ISOLATION-STAGE-2.
- DOCKER-ISOLATION-STAGE-2 is unused.

The only difference between `bridgeICC` and `bridgeNoICC` is the rule in the FORWARD
chain. To enable ICC, the rule for packets looping through the bridge is ACCEPT. For
no-ICC it's DROP.

[1]: usernet-portmap.md

And the corresponding nat table:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
