page_title: Docker HTTPS Setup
page_description: How to setup docker with https
page_keywords: docker, example, https, daemon

# Running Docker with https

By default, Docker runs via a non-networked Unix socket. It can also
optionally communicate using a HTTP socket.

If you need Docker reachable via the network in a safe manner, you can
enable TLS by specifying the tlsverify flag and pointing Docker’s
tlscacert flag to a trusted CA certificate.

In daemon mode, it will only allow connections from clients
authenticated by a certificate signed by that CA. In client mode, it
will only connect to servers with a certificate signed by that CA.

> **Warning**: 
> Using TLS and managing a CA is an advanced topic. Please make you self
> familiar with openssl, x509 and tls before using it in production.

## Create a CA, server and client keys with OpenSSL

First, initialize the CA serial file and generate CA private and public
keys:

    $ echo 01 > ca.srl
    $ openssl genrsa -des3 -out ca-key.pem
    $ openssl req -new -x509 -days 365 -key ca-key.pem -out ca.pem

Now that we have a CA, you can create a server key and certificate
signing request. Make sure that "Common Name (e.g. server FQDN or YOUR
name)" matches the hostname you will use to connect to Docker or just
use ‘\*’ for a certificate valid for any hostname:

    $ openssl genrsa -des3 -out server-key.pem
    $ openssl req -new -key server-key.pem -out server.csr

Next we’re going to sign the key with our CA:

    $ openssl x509 -req -days 365 -in server.csr -CA ca.pem -CAkey ca-key.pem \
      -out server-cert.pem

For client authentication, create a client key and certificate signing
request:

    $ openssl genrsa -des3 -out client-key.pem
    $ openssl req -new -key client-key.pem -out client.csr

To make the key suitable for client authentication, create a extensions
config file:

    $ echo extendedKeyUsage = clientAuth > extfile.cnf

Now sign the key:

    $ openssl x509 -req -days 365 -in client.csr -CA ca.pem -CAkey ca-key.pem \
      -out client-cert.pem -extfile extfile.cnf

Finally you need to remove the passphrase from the client and server
key:

    $ openssl rsa -in server-key.pem -out server-key.pem
    $ openssl rsa -in client-key.pem -out client-key.pem

Now you can make the Docker daemon only accept connections from clients
providing a certificate trusted by our CA:

    $ sudo docker -d --tlsverify --tlscacert=ca.pem --tlscert=server-cert.pem --tlskey=server-key.pem \
      -H=0.0.0.0:4243

To be able to connect to Docker and validate its certificate, you now
need to provide your client keys, certificates and trusted CA:

    $ docker --tlsverify --tlscacert=ca.pem --tlscert=client-cert.pem --tlskey=client-key.pem \
      -H=dns-name-of-docker-host:4243

> **Warning**: 
> As shown in the example above, you don’t have to run the
> `docker` client with `sudo` or
> the `docker` group when you use certificate
> authentication. That means anyone with the keys can give any
> instructions to your Docker daemon, giving them root access to the
> machine hosting the daemon. Guard these keys as you would a root
> password!

## Other modes

If you don’t want to have complete two-way authentication, you can run
Docker in various other modes by mixing the flags.

### Daemon modes

-   tlsverify, tlscacert, tlscert, tlskey set: Authenticate clients
-   tls, tlscert, tlskey: Do not authenticate clients

### Client modes

-   tls: Authenticate server based on public/default CA pool
-   tlsverify, tlscacert: Authenticate server based on given CA
-   tls, tlscert, tlskey: Authenticate with client certificate, do not
    authenticate server based on given CA
-   tlsverify, tlscacert, tlscert, tlskey: Authenticate with client
    certificate, authenticate server based on given CA

The client will send its client certificate if found, so you just need
to drop your keys into \~/.docker/\<ca, cert or key\>.pem
