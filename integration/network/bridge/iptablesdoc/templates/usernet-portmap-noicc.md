## Container on a user-defined network with inter-container communication disabled, with a published port

Equivalent to:

	docker network create \
	  -o com.docker.network.bridge.name=bridge1 \
	  -o com.docker.network.bridge.enable_icc=false \
	  --subnet 192.0.2.0/24 --gateway 192.0.2.1 bridge1
	docker run --network bridge1 -p 8080:80 --name c1 busybox

The filter table is:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

</details>

By comparison with [ICC=true][1]:

  - Rules 6 and 7 replace the accept rule for outgoing packets.
    - Rule 6, added by `setIcc`, drops any packet sent from the internal network to itself.
    - Rule 7, added by `setupIPTablesInternal` accepts any other outgoing packet.

[1]: usernet-portmap.md

And the corresponding nat table:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
