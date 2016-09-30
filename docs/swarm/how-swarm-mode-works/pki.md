<!--[metadata]>
+++
title = "How PKI works"
description = "How PKI works in swarm mode"
keywords = ["docker", "container", "cluster", "swarm mode", "node", "tls", "pki"]
[menu.main]
identifier="how-pki-work"
parent="how-swarm-works"
weight="5"
+++
<![end-metadata]-->

# How PKI works in swarm mode

The swarm mode public key infrastructure (PKI) system built into Docker Engine
makes it simple to securely deploy a container orchestration system. The nodes
in a swarm use mutual Transport Layer Security (TLS) to authenticate, authorize,
and encrypt the communications between themselves and other nodes in the swarm.

When you create a swarm by running `docker swarm init`, the Docker Engine
designates itself as a manager node. By default, the manager node generates
itself a new root Certificate Authority (CA) along with a key pair to secure
communications with other nodes that join the swarm. If you prefer, you can pass
the `--external-ca` flag to specify a root CA external to the swarm. Refer to
the [docker swarm init](../../reference/commandline/swarm_init.md) CLI
reference.

The manager node also generates two tokens to use when you join additional nodes
to the swarm: one worker token and one manager token. Each token includes the
digest of the root CA's certificate and a randomly generated secret. When a node
joins the swarm, it uses the digest to validate the root CA certificate from the
remote manager. It uses the secret to ensure the node is an approved node.

Each time a new node joins the swarm, the manager issues a certificate to the
node that contains a randomly generated node id to identify the node under the
certificate common name (CN) and the role under the organizational unit (OU).
The node id serves as the cryptographically secure node identity for the
lifetime of the node in the current swarm.

The diagram below illustrates how worker manager nodes and worker nodes encrypt
communications using a minimum of TLS 1.2.

![tls diagram](../images/tls.png)


The example below shows the information from a certificate from a worker node:

```bash
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            3b:1c:06:91:73:fb:16:ff:69:c3:f7:a2:fe:96:c1:73:e2:80:97:3b
        Signature Algorithm: ecdsa-with-SHA256
        Issuer: CN=swarm-ca
        Validity
            Not Before: Aug 30 02:39:00 2016 GMT
            Not After : Nov 28 03:39:00 2016 GMT
        Subject: O=ec2adilxf4ngv7ev8fwsi61i7, OU=swarm-worker, CN=dw02poa4vqvzxi5c10gm4pq2g
...snip...
```

By default, each node in the swarm renews its certificate every three months.
You can run `docker swarm update --cert-expiry <TIME PERIOD>` to configure the
frequency for nodes to renew their certificates. The minimum rotation value is 1
hour. Refer to the [docker swarm update](../../reference/commandline/swarm_update.md)
CLI reference.

## Learn More

* Read about how [nodes](nodes.md) work.
* Learn how swarm mode [services](services.md) work.
