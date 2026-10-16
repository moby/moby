Networking test-surface specifications
======================================

Each document here specifies a part of Moby's networking as it is observable from outside the
daemon. A document states:

- what that part is expected to do
- what a failing test means
- the vantage point from which to observe it
- what the verdict of a test depends on

| Spec | Scope |
| --- | --- |
| [network-common.md](network-common.md) | What every network driver must do, and the Engine API requests for networks and endpoints that do not depend on the driver. A stub. |
| [swarm-networking.md](swarm-networking.md) | The overlay network driver and the Swarm service mesh |
| [ipvlan-networking.md](ipvlan-networking.md) | The ipvlan network driver. A stub. |
| [bridge-networking.md](bridge-networking.md) | The bridge network driver, and the parts of the daemon's firewall integration that are not specific to Swarm. A stub. |
| [container-dns.md](container-dns.md) | Name resolution inside a container on a user-defined network, whatever the network's driver. A stub. |

A spec describes behavior only. It should change when the behavior of the daemon changes. It should
also change when a regression reveals an expectation that, until then, was only an assumption.

## Functional and acceptance tests

The specs drive two kinds of test. Their purposes tell them apart:

- Functional tests serve the maintainer's story: *"as a Moby maintainer, catch regressions in code
  changes."* Every expectation has one. They run wherever Moby's CI can run them. They can use any
  access the test harness gives them, such as daemon flags, the daemon log, restarting or killing
  the daemon, and failpoints. They should be cheap enough to run on every pull request.
- Acceptance tests serve the packager's story: *"as a packager, verify a candidate release works on
  every platform in my support matrix."* They run on each platform in that matrix, against the
  daemon as packaged.

What an expectation depends on decides whether it also needs acceptance tests. See
[Depends on](#depends-on).

## Expectations

A spec is a series of numbered sections. Each section has a table of expectations. A row gives the
state that the expectation applies to, the expectation, its tier, the vantage point from which to
observe it, and what its verdict depends on. After the numbered sections, a spec can list its known
divergences & non-goals, and what it deliberately does not assert.

The "Applies to" cell gives the state that a test sets up before the row's action. Examples are
objects that exist, daemon settings, the platform, and what happened to an object before the action.
It never names the action itself. A request, a command, an update or a restart that the row is about
goes in the "Expectation" cell. When the cell is blank, the row applies whenever its section does.

An expectation states the outcome of an action at the same level of abstraction as the action. The
outcome of an Engine API request is the API's response: its status and body. The outcome of a
`docker` command is what the command prints and its exit status. An HTTP status is not visible at
the level of a `docker` command. A row about a `docker` command names `cli` among what it depends
on. It usually has a partner row about the API request that the command makes.

An outcome covers only the direct response to the action. A side effect is not part of the outcome.
Examples of side effects are a network created or removed, traffic on the wire, and an entry in the
daemon's log. A spec states a side effect where it can be observed, whatever the action was. An
expectation that an action succeeds also states the side effect that shows the action took effect. A
daemon that reports success and does nothing fails such an expectation.

Unless the row names a range, an expectation about an Engine API request holds at every API version
that the daemon accepts by default. These versions go from the daemon's default minimum version to
the current version.

A row names a daemon setting by its `daemon.json` key, such as `firewall-backend` or
`features.swarm-nftables`. The matching `dockerd` flag, such as `--firewall-backend`, sets the same
setting. A row holds whichever of the two sets it. A flag that a row names, such as `--dns`, is a
flag of a `docker` command.

### Tier

Every expectation carries a tier. The tier says what a failing test means and how to respond to it.

| Tier | A failure means |
| --- | --- |
| B (behavioral) | *You broke this.* User-visible semantics. A change needs justification. |
| C (characterization) | *You changed this. Was that intended?* Internal but load-bearing. A deliberate change is fine. An accidental change is not. |
| (omitted) | Incidental. The spec does not assert it at all. Each spec lists such behaviors under *Deliberately not asserted*. |

A behavior can carry both tiers, as a B row and a C row. The inner MTU in
[swarm-networking.md §3](swarm-networking.md#3-observable-state-inside-a-container) is the model
case: it is B as a property of the path and C as a pinned value.

### Observed from

A black-box expectation means something only from a stated vantage point. Some rows hold from one
vantage point and not from another.

| Code | Vantage point |
| --- | --- |
| `task` | Inside a Swarm service task's container |
| `ctr` | Inside a standalone container, one that is not a Swarm service task. A spec can narrow which networks the container is attached to. |
| `node` | The host network namespace of a node running the daemon |
| `underlay` | The physical network between nodes. A test can capture its traffic at any host on it, including a node's own network interfaces. A test can send or inject traffic from any host on the physical network other than the node under test. Examples are another node, and a container outside the node under test that is attached to the physical network. |
| `client` | A host other than the node under test that can reach one of the node's addresses. It can be another node of the same Swarm. |
| `api:mgr` | Engine API / CLI against a Swarm manager |
| `api:wkr` | Engine API / CLI against a Swarm worker |
| `api:active` | Engine API / CLI against a Swarm node, manager or worker, whose availability is `active`. Only such a node can run a Swarm service task or attach a standalone container to a Swarm-scoped network. |
| `api:solo` | Engine API / CLI against a daemon that is not in a Swarm |
| `api` | Engine API / CLI against a daemon, whether or not it is in a Swarm |
| `log` | The daemon log on a node. It is the only channel for a whole class of partial failure. See [swarm-networking.md §11.6](swarm-networking.md#116-failure-reporting) |

In a row, `a` → `b` names two vantage points: traffic sent from `a` is observed at `b`.

Where a row refers to the node's address, a test can use any address assigned to an interface in the
node's host network namespace.

### Depends on

A row names the facilities its verdict depends on, or `none`. A code can have a platform in
parentheses after it, such as `os` (Windows). Such a code applies on nodes of that platform only. On
the other nodes, the row names `none`.

| Code | Facility | What varies |
| --- | --- | --- |
| `os` | What the operating system vendor supplies: the kernel and first-party userland | On Linux: the kernel's netfilter, IPVS, XFRM, VXLAN and SCTP support, nftables and iptables userland, the iptables-nft shim, firewalld, systemd-resolved or NetworkManager, and sysctl defaults. On Windows: the Host Networking Service, the Virtual Filtering Platform and the DNS client. Which features each OS release offers, and how they behave. |
| `upgrade` | A package's upgrade path from the previous release | Its service units and maintainer scripts, which stop, start or restart the daemon across the upgrade |
| `cli` | The `docker` CLI a packager ships with the daemon | Its version, how it turns commands and flags into API requests, and how it prints the responses |

The rule:

> Every expectation gets a functional test. An expectation that names a facility also gets an
> acceptance test on every platform in the support matrix, whatever its tier. An expectation that
> names `none` does not get an acceptance test, for two reasons:
>
> - Its verdict cannot differ between platforms.
> - A packager who changes the daemon runs the functional tests against their own build.

A row names a facility if the facility decides the verdict when a test uses the real facility. A
test that replaces the facility with a failpoint or a mock does not depend on it. A row whose only
test replaces the facility names `none`.

### Platforms

Unless a section or a row narrows it, an expectation holds on Linux and Windows nodes alike. Unless
a section or a row narrows it, an expectation also holds for traffic between nodes of any mix of the
two. A section built on a Linux-only mechanism says so in its introduction. A row that depends on a
Linux-only daemon setting, such as the userland proxy or the firewall backend, holds on Linux nodes.
The spec lists a platform on which an expectation does not hold yet as a known divergence.
