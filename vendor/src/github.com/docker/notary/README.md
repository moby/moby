# Notary 
[![Circle CI](https://circleci.com/gh/docker/notary/tree/master.svg?style=shield)](https://circleci.com/gh/docker/notary/tree/master) [![CodeCov](https://codecov.io/github/docker/notary/coverage.svg?branch=master)](https://codecov.io/github/docker/notary)

The Notary project comprises a [server](cmd/notary-server) and a [client](cmd/notary) for running and interacting
with trusted collections.


Notary aims to make the internet more secure by making it easy for people to
publish and verify content. We often rely on TLS to secure our communications
with a web server which is inherently flawed, as any compromise of the server
enables malicious content to be substituted for the legitimate content.

With Notary, publishers can sign their content offline using keys kept highly
secure. Once the publisher is ready to make the content available, they can
push their signed trusted collection to a Notary Server.

Consumers, having acquired the publisher's public key through a secure channel,
can then communicate with any notary server or (insecure) mirror, relying
only on the publisher's key to determine the validity and integrity of the
received content.

## Goals

Notary is based on [The Update Framework](http://theupdateframework.com/), a secure general design for the problem of software distribution and updates. By using TUF, notary achieves a number of key advantages:

* **Survivable Key Compromise**: Content publishers must manage keys in order to sign their content. Signing keys may be compromised or lost so systems must be designed in order to be flexible and recoverable in the case of key compromise. TUF's notion of key roles is utilized to separate responsibilities across a hierarchy of keys such that loss of any particular key (except the root role) by itself is not fatal to the security of the system.
* **Freshness Guarantees**: Replay attacks are a common problem in designing secure systems, where previously valid payloads are replayed to trick another system. The same problem exists in the software update systems, where old signed can be presented as the most recent. notary makes use of timestamping on publishing so that consumers can know that they are receiving the most up to date content. This is particularly important when dealing with software update where old vulnerable versions could be used to attack users.
* **Configurable Trust Thresholds**: Oftentimes there are a large number of publishers that are allowed to publish a particular piece of content. For example, open source projects where there are a number of core maintainers. Trust thresholds can be used so that content consumers require a configurable number of signatures on a piece of content in order to trust it. Using thresholds increases security so that loss of individual signing keys doesn't allow publishing of malicious content.
* **Signing Delegation**: To allow for flexible publishing of trusted collections, a content publisher can delegate part of their collection to another signer. This delegation is represented as signed metadata so that a consumer of the content can verify both the content and the delegation.
* **Use of Existing Distribution**: Notary's trust guarantees are not tied at all to particular distribution channels from which content is delivered. Therefore, trust can be added to any existing content delivery mechanism.
* **Untrusted Mirrors and Transport**: All of the notary metadata can be mirrored and distributed via arbitrary channels.

# Notary CLI

Notary is a tool for publishing and managing trusted collections of content. Publishers can digitally sign collections and consumers can verify integrity and origin of content. This ability is built on a straightforward key management and signing interface to create signed collections and configure trusted publishers.

## Using Notary
Lets try using notary.

Prerequisites:

- Requirements from the [Compiling Notary Server](#compiling-notary-server) section (such as go 1.5.1)
- [docker and docker-compose](http://docs.docker.com/compose/install/)
- [Notary server configuration](#configuring-notary-server)

As setup, let's build notary and then start up a local notary-server (don't forget to add `127.0.0.1 notary-server` to your `/etc/hosts`, or if using docker-machine, add `$(docker-machine ip) notary-server`).

```sh
make binaries
docker-compose build
docker-compose up -d
```

Note: In order to have notary use the local notary server and development root CA we can load the local development configuration by appending `-c cmd/notary/config.json` to every command. If you would rather not have to use `-c` on every command, copy `cmd/notary/config.json and cmd/notary/root-ca.crt` to `~/.notary`.


First, let's initiate a notary collection called `example.com/scripts`

```sh
notary init example.com/scripts
```

Now, look at the keys you created as a result of initialization
```sh
notary key list
```

Cool, now add a local file `install.sh` and call it `v1`
```sh
notary add example.com/scripts v1 install.sh
```

Wouldn't it be nice if others could know that you've signed this content? Use `publish` to publish your collection to your default notary-server
```sh
notary publish example.com/scripts
```

Now, others can pull your trusted collection
```sh
notary list example.com/scripts
```

More importantly, they can verify the content of your script by using `notary verify`:
```sh
curl example.com/install.sh | notary verify example.com/scripts v1 | sh
```

# Notary Server

Notary Server manages TUF data over an HTTP API compatible with the
[notary client](cmd/notary).

It may be configured to use either JWT or HTTP Basic Auth for authentication.
Currently it only supports MySQL for storage of the TUF data, we intend to
expand this to other storage options.

## Setup for Development

The notary repository comes with Dockerfiles and a docker-compose file
to facilitate development. Simply run the following commands to start
a notary server with a temporary MySQL database in containers:

```
$ docker-compose build
$ docker-compose up
```

If you are on Mac OSX with boot2docker or kitematic, you'll need to
update your hosts file such that the name `notary` is associated with
the IP address of your VM (for boot2docker, this can be determined
by running `boot2docker ip`, with kitematic, `echo $DOCKER_HOST` should
show the IP of the VM). If you are using the default Linux setup,
you need to add `127.0.0.1 notary` to your hosts file.

## Successfully connecting over TLS

By default notary-server runs with TLS with certificates signed by a local
CA. In order to be able to successfully connect to it using
either `curl` or `openssl`, you will have to use the root CA file in `fixtures/root-ca.crt`.

OpenSSL example:

`openssl s_client -connect notary-server:4443 -CAfile fixtures/root-ca.crt`

## Compiling Notary Server

Prerequisites:

- Go = 1.5.1
- [godep](https://github.com/tools/godep) installed
- libtool development headers installed

Install dependencies by running `godep restore`.

From the root of this git repository, run `make binaries`. This will
compile the `notary`, `notary-server`, and `notary-signer` applications and
place them in a `bin` directory at the root of the git repository (the `bin`
directory is ignored by the .gitignore file).

`notary-signer` depends upon `pkcs11`, which requires that libtool headers be installed (`libtool-dev` on Ubuntu, `libtool-ltdl-devel` on CentOS/RedHat). If you are using Mac OS, you can `brew install libtool`, and run `make binaries` with the following environment variables (assuming a standard installation of Homebrew):

```sh
export CPATH=/usr/local/include:${CPATH}
export LIBRARY_PATH=/usr/local/lib:${LIBRARY_PATH}
```

## Running Notary Server

The `notary-server` application has the following usage:

```
$ bin/notary-server --help
usage: bin/notary-serve
  -config="": Path to configuration file
  -debug=false: Enable the debugging server on localhost:8080
```

## Configuring Notary Server

The configuration file must be a json file with the following format:

```json
{
    "server": {
        "addr": ":4443",
        "tls_cert_file": "./fixtures/notary-server.crt",
        "tls_key_file": "./fixtures/notary-server.key"
    },
    "logging": {
        "level": 5
    }
}
```

The pem and key provided in fixtures are purely for local development and
testing. For production, you must create your own keypair and certificate,
either via the CA of your choice, or a self signed certificate.

If using the pem and key provided in fixtures, either:
- Add `fixtures/root-ca.crt` to your trusted root certificates
- Use the default configuration for notary client that loads the CA root for you by using the flag `-c ./cmd/notary/config.json`
- Disable TLS verification by adding the following option notary configuration file in `~/.notary/config.json`:

            "skipTLSVerify": true

Otherwise, you will see TLS errors or X509 errors upon initializing the
notary collection:

```
$ notary list diogomonica.com/openvpn
* fatal: Get https://notary-server:4443/v2/: x509: certificate signed by unknown authority
$ notary list diogomonica.com/openvpn -c cmd/notary/config.json
latest b1df2ad7cbc19f06f08b69b4bcd817649b509f3e5420cdd2245a85144288e26d 4056
```
