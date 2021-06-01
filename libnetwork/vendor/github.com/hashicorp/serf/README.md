# Serf

* Website: https://www.serfdom.io
* IRC: `#serfdom` on Freenode
* Mailing list: [Google Groups](https://groups.google.com/group/serfdom/)

Serf is a decentralized solution for service discovery and orchestration
that is lightweight, highly available, and fault tolerant.

Serf runs on Linux, Mac OS X, and Windows. An efficient and lightweight gossip
protocol is used to communicate with other nodes. Serf can detect node failures
and notify the rest of the cluster. An event system is built on top of
Serf, letting you use Serf's gossip protocol to propagate events such
as deploys, configuration changes, etc. Serf is completely masterless
with no single point of failure.

Here are some example use cases of Serf, though there are many others:

* Discovering web servers and automatically adding them to a load balancer
* Organizing many memcached or redis nodes into a cluster, perhaps with
  something like [twemproxy](https://github.com/twitter/twemproxy) or
  maybe just configuring an application with the address of all the
  nodes
* Triggering web deploys using the event system built on top of Serf
* Propagating changes to configuration to relevant nodes.
* Updating DNS records to reflect cluster changes as they occur.
* Much, much more.

## Quick Start

First, [download a pre-built Serf binary](https://www.serfdom.io/downloads.html)
for your operating system or [compile Serf yourself](#developing-serf).

Next, let's start a couple Serf agents. Agents run until they're told to quit
and handle the communication of maintenance tasks of Serf. In a real Serf
setup, each node in your system will run one or more Serf agents (it can
run multiple agents if you're running multiple cluster types. e.g. web
servers vs. memcached servers).

Start each Serf agent in a separate terminal session so that we can see
the output of each. Start the first agent:

```
$ serf agent -node=foo -bind=127.0.0.1:5000 -rpc-addr=127.0.0.1:7373
...
```

Start the second agent in another terminal session (while the first is still
running):

```
$ serf agent -node=bar -bind=127.0.0.1:5001 -rpc-addr=127.0.0.1:7374
...
```

At this point two Serf agents are running independently but are still
unaware of each other. Let's now tell the first agent to join an existing
cluster (the second agent). When starting a Serf agent, you must join an
existing cluster by specifying at least one existing member. After this,
Serf gossips and the remainder of the cluster becomes aware of the join.
Run the following commands in a third terminal session.

```
$ serf join 127.0.0.1:5001
...
```

If you're watching your terminals, you should see both Serf agents
become aware of the join. You can prove it by running `serf members`
to see the members of the Serf cluster:

```
$ serf members
foo    127.0.0.1:5000    alive
bar    127.0.0.1:5001    alive
...
```

At this point, you can ctrl-C or force kill either Serf agent, and they'll
update their membership lists appropriately. If you ctrl-C a Serf agent,
it will gracefully leave by notifying the cluster of its intent to leave.
If you force kill an agent, it will eventually (usually within seconds)
be detected by another member of the cluster which will notify the
cluster of the node failure.

## Documentation

Full, comprehensive documentation is viewable on the Serf website:

https://www.serfdom.io/docs

## Developing Serf

If you wish to work on Serf itself, you'll first need [Go](https://golang.org)
installed (version 1.2+ is _required_). Make sure you have Go properly 
[installed](https://golang.org/doc/install),
including setting up your [GOPATH](https://golang.org/doc/code.html#GOPATH).

Next, clone this repository into `$GOPATH/src/github.com/hashicorp/serf` and
then just type `make`. In a few moments, you'll have a working `serf` executable:

```
$ make
...
$ bin/serf
...
```

*note: `make` will also place a copy of the executable under $GOPATH/bin*

You can run tests by typing `make test`.

If you make any changes to the code, run `make format` in order to automatically
format the code according to Go standards.
