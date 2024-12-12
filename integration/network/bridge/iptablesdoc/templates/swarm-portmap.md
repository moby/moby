## Swarm service, with a published port

Equivalent to:

	docker service create -p 8080:80 busybox top

The filter table is:

    {{index . "LFilter4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SFilter4"}}

</details>

Note that:

 - There's a bridge network called `docker_gwbridge` for swarm ingress.
   - Its rules follow the usual pattern for a network with inter-container communication disabled.
- There's an additional chain `DOCKER-INGRESS`.
  - The jump to `DOCKER-INGRESS` is in the `FORWARD` chain, after the jump to `DOCKER-USER`.

And the corresponding nat table:

    {{index . "LNat4"}}

<details>
<summary>iptables commands</summary>

    {{index . "SNat4"}}

</details>
