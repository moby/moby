Networking test-surface specifications
======================================

Each document here specifies part of Moby's networking as observable from outside the daemon: what
it is expected to do, what a failing test means, where it is observed from, and what its verdict
depends on.

| Spec | Scope |
| --- | --- |
| [swarm-networking.md](swarm-networking.md) | The overlay network driver and the Swarm service mesh |
| [bridge-networking.md](bridge-networking.md) | The bridge network driver, and the parts of the daemon's firewall integration that are not specific to Swarm. A stub. |
| [container-dns.md](container-dns.md) | Name resolution inside a container on a user-defined network, whatever the network's driver. A stub. |

A spec describes behavior only. It should change when the daemon's behavior does, or when a
regression reveals an expectation that was previously only assumed.

## Functional and acceptance tests

The specs drive two kinds of test, told apart by their purpose:

- Functional tests serve the maintainer's story: *"as a Moby maintainer, catch regressions in code
  changes."* Every expectation has one. They run wherever Moby's CI can run them, and can use any
  access the test harness gives them, such as daemon flags, the daemon log, restarting or killing
  the daemon, and failpoints. They should be cheap enough to run on every pull request.
- Acceptance tests serve the packager's story: *"as a packager, verify a candidate release works on
  every platform in my support matrix."* They run on each platform in that matrix, against the
  daemon as packaged.

What an expectation depends on decides whether it also needs acceptance tests; see
[Depends on](#depends-on).

## Expectations

A spec is a series of numbered sections, each with a table of expectations. A row gives the
expectation, its tier, the vantage point it is observed from, and what its verdict depends on. After
the numbered sections, a spec can list its known divergences & non-goals, and what it deliberately
does not assert.

An expectation states the outcome of an action at the same level of abstraction as the action. The
outcome of an Engine API request is the API's response: its status and body. The outcome of a
`docker` command is what the command prints and its exit status, where an HTTP status is not
visible. This covers only the direct response to the action. A side effect, such as a network
created or removed, traffic on the wire or an entry in the daemon's log, is stated where it can be
observed, whatever the action was. An expectation that an action succeeds also states the side
effect that shows the action took effect. A daemon that reports success and does nothing fails such
an expectation. A row about a `docker` command names `cli` among what it depends on and usually has
a partner row about the API request the command makes.

An expectation about an Engine API request holds at every API version the daemon accepts by default,
from its default minimum to the current version, unless the row names a range.

### Tier

Every expectation carries a tier. The tier says what a failing test means and how to respond to it.

| Tier | A failure means |
| --- | --- |
| B (behavioral) | *You broke this.* User-visible semantics; a change needs justification. |
| C (characterization) | *You changed this; was that intended?* Internal but load-bearing. A deliberate change is fine; an accidental one is not. |
| (omitted) | Incidental. Not asserted at all. Each spec lists these under *Deliberately not asserted*. |

A behavior can carry both tiers, as a B row and a C row. The inner MTU in
[swarm-networking.md §3](swarm-networking.md#3-observable-state-inside-a-container) is the model
case: it is B as a property of the path and C as a pinned value.

### Observed from

A black-box expectation only means something from a stated vantage point, and some rows hold
from one vantage point and not another.

| Code | Vantage point |
| --- | --- |
| `task` | Inside a Swarm service task's container |
| `ctr` | Inside a standalone container, one that is not a Swarm service task. A spec can narrow which networks it is attached to. |
| `node` | The host network namespace of a node running the daemon |
| `underlay` | The physical network between nodes. Its traffic can be captured at any host on it, a node's own network interfaces included, and sent or injected from any host other than the node under test, such as another node or a container attached directly to the physical network. |
| `client` | An external client host reaching a published port |
| `api:mgr` | Engine API / CLI against a Swarm manager |
| `api:wkr` | Engine API / CLI against a Swarm worker |
| `api` | Engine API / CLI against a daemon, whether or not it is in a Swarm |
| `log` | The daemon log on a node. It is the only channel for a whole class of partial failure; see [swarm-networking.md §11.6](swarm-networking.md#116-failure-reporting) |

In a row, `a` → `b` names two vantage points: traffic sent from `a` is observed at `b`.

Where a row refers to the node's address, any address assigned to an interface in the node's host
network namespace will do.

### Depends on

A row names the facilities its verdict depends on, or `none`. A code followed by a platform in
parentheses, such as `os` (Windows), applies on nodes of that platform only, and the row names
`none` on the others.

| Code | Facility | What varies |
| --- | --- | --- |
| `os` | What the operating system vendor supplies: the kernel and first-party userland | On Linux: the kernel's netfilter, IPVS, XFRM, VXLAN and SCTP support, nftables and iptables userland, the iptables-nft shim, firewalld, systemd-resolved or NetworkManager, and sysctl defaults. On Windows: the Host Networking Service, the Virtual Filtering Platform and the DNS client. Which features each OS release offers, and how they behave. |
| `upgrade` | A package's upgrade path from the previous release | Its service units and maintainer scripts, which stop, start or restart the daemon across the upgrade |
| `cli` | The `docker` CLI a packager ships with the daemon | Its version, how it turns commands and flags into API requests, and how it prints the responses |

The rule:

> Every expectation gets a functional test. An expectation that names a facility also gets an
> acceptance test on every platform in the support matrix, whatever its tier. An expectation that
> names `none` is not tested for acceptance: its verdict cannot differ between platforms, and a
> packager who changes the daemon runs the functional tests against their own build.

A row names a facility when the facility decides the verdict as a test exercises it for real. A test
that stands in for the facility with a failpoint or a mock does not depend on it. A row whose only
test stands in for the facility names `none`.

### Platforms

Unless a section or a row narrows it, an expectation holds on Linux and Windows nodes alike, and for
traffic between nodes of any mix of the two. A section built on a Linux-only mechanism says so in
its introduction. A row that depends on a Linux-only daemon setting, such as the userland proxy or
the firewall backend, holds on Linux nodes. A platform on which an expectation does not hold yet is
listed as a known divergence.
