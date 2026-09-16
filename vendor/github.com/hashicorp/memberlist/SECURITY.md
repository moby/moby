# memberlist Security Model

This document describes the security model of `memberlist`: the guarantees it
provides, the configuration required to obtain them, and
what is explicitly **outside** its threat model.

`memberlist` is a library, not a product. It is embedded by higher-level systems
such as [HashiCorp Serf](https://github.com/hashicorp/serf), and, through Serf,
by [Consul](https://developer.hashicorp.com/consul) and
[Nomad](https://developer.hashicorp.com/nomad). This document is written for
developers integrating `memberlist`. Some responsibilities described here
(identity, authorization, key distribution, encryption at rest) are intentionally
delegated to the embedding application.

> [!IMPORTANT]
> `memberlist` is **not secure by default**. Without a gossip encryption key
> configured, all membership traffic is transmitted in plaintext and is neither
> encrypted nor authenticated. In that mode, any host that can reach the gossip
> port can read membership information and forge membership messages.

## Overview

`memberlist` manages cluster membership and node failure detection using a
gossip protocol based on
[SWIM](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf).
Nodes exchange two kinds of traffic:

- **UDP packets** — probes, acks, and gossiped state changes (`alive`,
  `suspect`, `dead`, user messages).
- **TCP streams** — full-state push/pull synchronization and TCP fallback pings.

The only security primitive `memberlist` provides is symmetric encryption and
authentication of this traffic using a shared keyring (AES-GCM). It does not
provide, and is not designed to provide, per-node identity, authentication, or
authorization.

### Trust model

The trust boundary of a `memberlist` cluster is **possession of a current
gossip key**.

- Every node in a cluster shares the same symmetric key material (the keyring).
- Any party holding a current key is treated as a fully trusted member of the
  cluster. There is no cryptographic notion of an individual node's identity,
  and therefore no way to distinguish a legitimate member from a malicious one
  that also holds the key.
- `memberlist` is a weakly consistent group membership protocol. It is **not
  Byzantine fault tolerant** and is **not a consensus protocol**. A single
  authenticated-but-malicious peer can disrupt the cluster:
  it can add nodes, remove nodes (by broadcasting `dead`/`suspect`), reclaim or
  impersonate a node's name (by broadcasting a higher incarnation number), and
  alter node metadata.

This is why systems built on `memberlist` (Serf, Consul, Nomad) document gossip
encryption as a **requirement** for a secure cluster, and layer their own
identity and authorization systems (e.g. mTLS and ACLs) on top.

## Secure configuration

`memberlist`'s security properties only apply when it is configured correctly.

### Requirements

- **Enable gossip encryption.** Provide a key via `Config.SecretKey` (or a
  fully constructed `Config.Keyring`). The key must be 16, 24, or 32 bytes,
  selecting AES-128, AES-192, or AES-256 respectively. When a key is present,
  `memberlist` encrypts and authenticates all UDP gossip and all TCP
  push/pull state synchronization using AES-GCM.
- **Keep verification enforced.** `Config.GossipVerifyIncoming` and
  `Config.GossipVerifyOutgoing` both default to `true` and should remain so.
  - `GossipVerifyIncoming = true` causes messages that cannot be decrypted and
    authenticated to be dropped. Setting `GossipVerifyIncoming = false` makes a node
    accept plaintext messages even when a key is configured.
  - `GossipVerifyOutgoing = true` causes all outbound messages to be encrypted.
  - These two flags exist solely to allow a running cluster to migrate from
    plaintext to encrypted gossip without downtime, and should be returned to `true`
    once migration is complete.
- **Protect the gossip key as a cluster-wide shared secret.** Distribute it
  out-of-band over a secure channel and restrict access to it. Anyone with the
  key is a fully trusted cluster member.
- **Rotate keys periodically.** Use the keyring API
  (`Keyring.AddKey`, `Keyring.UseKey`, `Keyring.RemoveKey`) to rotate the
  primary key. Additional keys remain valid for decryption during rollout, which
  supports zero-downtime rotation.

### Recommendations

- **Restrict network exposure.** The
  [`DefaultLANConfig`](https://pkg.go.dev/github.com/hashicorp/memberlist#DefaultLANConfig)
  and
  [`DefaultWANConfig`](https://pkg.go.dev/github.com/hashicorp/memberlist#DefaultWANConfig)
  helpers bind `memberlist` to `0.0.0.0` on port `7946` (TCP and UDP). Firewall
  the gossip port so it is reachable only by legitimate cluster members.
- **Use `Config.CIDRsAllowed`.** This restricts the source networks from which
  a node will accept connections and which addresses may be added to the
  membership list. It is defense-in-depth, not a substitute for encryption.
- **Set a `Config.Label`.** When encryption is enabled, the label is bound to
  every message as GCM Additional Authenticated Data (AAD), which prevents nodes
  from accepting messages intended for a different logical cluster that uses a
  different label. The label is **not** a secret and provides no protection when
  encryption is disabled.
- **Use `Config.RequireNodeNames`** to require node names on messages where
  appropriate for your deployment.
- **Validate at the application layer.** Use the `Alive`, `Conflict`, and
  `Merge` delegates to apply application-specific admission and conflict checks.
- **Layer identity and authorization above `memberlist`.** If you need per-node
  authentication or access control, implement it in the embedding application
  (as Serf/Consul/Nomad do with mTLS and ACLs). `memberlist` alone does not
  provide it.

## Threat model

When `memberlist` is configured securely (a gossip key is set and verification
is enforced), the following are considered part of its threat model:

- **Eavesdropping on gossip in transit.** All UDP and TCP gossip traffic is
  encrypted with AES-GCM, preventing an on-path observer without the key from
  reading membership information.
- **Tampering with messages in transit.** The GCM authentication tag detects
  modification; altered messages fail authentication and are discarded.
- **Forged or injected messages from parties without a key.** Messages that
  cannot be decrypted and authenticated with an installed key are rejected.
- **State corruption from malformed messages.** Improperly formatted messages
  are discarded; well-formed messages are only processed after successful
  authentication.
- **Cross-cluster message confusion.** When a `Label` is configured, it is
  authenticated as GCM AAD, so messages authenticated for another logical
  cluster are not accepted.
- **Bounded resource-exhaustion via oversized state.** Full-state sync is capped
  (`maxPushStateBytes`), decompression output is bounded, the inbound message
  handoff queue has a bounded depth, and `CIDRsAllowed` can constrain sources.
  These provide partial protection against certain resource-exhaustion attacks
  (see the exclusions below for the limits of this).

## Not in the threat model

The following are explicitly excluded from `memberlist`'s threat model.
Integrators must mitigate these at a higher layer or through operational
controls.

- **Malicious authenticated peers / key compromise.** Any party holding a
  current gossip key is fully trusted. Such a party can add, remove, impersonate,
  or alter members and otherwise disrupt the cluster. `memberlist` is not
  Byzantine fault tolerant. Mitigate through strict key custody, network
  segmentation, and higher-layer identity/authorization.
- **Per-node authentication and authorization.** `memberlist` has no concept of
  individual node identity, roles, capabilities, or access control. Systems that
  need these (for example Serf, Consul, and Nomad) provide them separately via
  mechanisms such as mTLS and ACLs.
- **Key distribution and exchange.** `memberlist` provides keyring primitives
  but does not include a secure mechanism for distributing or exchanging keys.
  Delivering keys to nodes securely is the integrator's responsibility.
- **Encryption at rest.** The keyring and current member state are held in
  process memory. `memberlist` does not encrypt keys or state on disk;
  protecting any persisted configuration or key material is the integrator's
  responsibility.
- **Host, process, and memory access.** An attacker who can read the memory of a
  running process, or otherwise gain access to the host, can recover the gossip
  keys and all membership state. This is outside the scope of the library.
- **Metadata confidentiality and traffic analysis.** Even with encryption
  enabled, observable properties of the traffic — message sizes, timing, packet
  framing, and the plaintext outer `Label` header — are not protected. They may
  leak information about cluster size and activity.
- **Replay protection as a designed control.** `memberlist` does not maintain a
  nonce or replay cache. SWIM incarnation numbers cause stale state transitions
  to be ignored, which provides incidental resistance to naive replay, but this
  is not a security guarantee against a capable attacker — particularly one that
  also holds a valid key.
- **Denial of service beyond the built-in bounds.** The built-in limits noted in
  the threat model reduce specific application-level resource-exhaustion vectors,
  but volumetric network floods, amplification, and OS/network-layer exhaustion
  are out of scope. Mitigate these with firewalls, rate limiting, and network
  controls.
- **Plaintext operation.** With no key configured — or with
  `GossipVerifyIncoming = false` — gossip traffic is unauthenticated and
  unencrypted, and none of the guarantees above apply. Plaintext mode is intended
  only for trusted, isolated networks or for temporary migration.
- **Custom transports.** When a custom `Config.Transport` is supplied, the
  security of that transport implementation is the responsibility of its author.

## Network ports

| Port | Protocol | Purpose |
| ---- | -------- | ------- |
| 7946 (default) | TCP | Full-state push/pull synchronization and TCP fallback pings. |
| 7946 (default) | UDP | Gossip: probes, acks, and membership state changes. |

The bind address and port are configurable via `Config.BindAddr` /
`Config.BindPort`, and the advertised address via `Config.AdvertiseAddr` /
`Config.AdvertisePort`.

## Reporting a vulnerability

Please do not report security vulnerabilities through public GitHub issues.
Report suspected vulnerabilities through the maintainer's coordinated
disclosure process. For the HashiCorp-maintained lineage of this project, see
<https://www.hashicorp.com/trust/security>.
