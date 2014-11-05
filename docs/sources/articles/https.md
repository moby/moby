page_title: Running Docker with HTTPS
page_description: How to setup and run Docker with HTTPS
page_keywords: docker, docs, article, example, https, daemon, tls, ca, certificate

# Running Docker with https

By default, Docker runs via a non-networked Unix socket. It can also
optionally communicate using a HTTP socket.  Enabling communication
through HTTP will by default use TLS and require daemon configuration
to allow client connections.


There are two different ways of authenticating connections between Docker
client and daemon, both of which use secure TLS connections.

 - **Identity-based authentication** uses an authorized keys list on the daemon
to whitelist client connections.  The client must also accept the daemon's key
and remember it for future connections.
 - **Certificate-based authentication** uses a certificate authority to
authorize connections.  Using this method requires additional setup to enable
client authentication.

The authentication method is selected using the `--auth` flag with values
 `identity`, `cert`, or `none` . `identity` is the default method and `none`
should only be used with caution.

## Identity-based authentication

Identity-based authentication is similar to how SSH does authentication. When
connecting to a daemon for the first time, a client will ask whether a user
trusts a fingerprint of the daemon’s public key. If they do, the public key will
be stored so it does not prompt on subsequent connections. For the daemon
to authenticate the client, each client automatically generates its own
key (~/.docker/key.json) which is presented to the daemon and checked
against a list of keys authorized to connect (~/.docker/authorized_keys.json).

To enable identity-based authentication, add the flag `--auth=identity`.
The default identity and authorization files may be overridden through the
flags:

 - `--identity` specifies the key file to use.  This file contains the client's
private key and its fingerprint is used by the daemon to identify the client.
This file should be secured.
 - `--auth-authorized-keys` - specifies the client whitelist.  This is a daemon
configuration and should have its write permissions restricted.
 - `--auth-allowed-host` - specifies the list of daemon public key fingerprints
which have been approved by the user and the host name associated with
each fingerprint.

## Certificate-based authentication

Certificate-based authentication uses TLS certificates provided by a
certificate authority. This is for advanced usage where you may want to
integrate Docker with other TLS-compatible tools or you may already use PKI
within your organisation. You can just get the client to verify the server’s
certificate against a CA, or do full two-way authentication by getting the
server to also verify the client’s certificate.

To enable certificate-based authentication, add the flag `--auth=cert` and
point the `--auth-ca` flag to a trusted CA certificate.

In the daemon mode, it will only allow connections from clients
authenticated by a certificate signed by that CA. In the client mode,
it will only connect to servers with a certificate signed by that CA.

### Client configuration

To enable certificate-based authentication, use the `--auth=cert` option. By
default, this will use the public CA pool. You want to use a specific CA,
specify its path with the `--auth-ca` option.

If the server requires client certificate authentication, you can provide this
with the `--auth-cert` and `--auth-key` options.

### Daemon configuration

When running the daemon with the `--auth=cert` option, it will serve a TLS
connection that the client can verify against its CA certificate. You must
provide a keypair for the client to check with the `--auth-cert` and
`--auth-key` options.

If you also want the client to authenticate with a client certificate, you can
pass a CA to check the certificate against with the `--auth-ca` option.

### Create a CA, server and client keys with OpenSSL

> **Warning**:
> Using TLS and managing a CA is an advanced topic. Please familiarize yourself
> with OpenSSL, x509 and TLS before using it in production.

> **Warning**:
> These TLS commands will only generate a working set of certificates on Linux.
> Mac OS X comes with a version of OpenSSL that is incompatible with the
> certificates that Docker requires.

### Create a CA, server and client keys with OpenSSL

First, initialize the CA serial file and generate CA private and public
keys:

    $ openssl genrsa -aes256 -out ca-key.pem 2048
    Generating RSA private key, 2048 bit long modulus
    ......+++
    ...............+++
    e is 65537 (0x10001)
    Enter pass phrase for ca-key.pem:
    Verifying - Enter pass phrase for ca-key.pem:
    $ openssl req -new -x509 -days 365 -key ca-key.pem -sha256 -out ca.pem
    Enter pass phrase for ca-key.pem:
     You are about to be asked to enter information that will be incorporated
     into your certificate request.
     What you are about to enter is what is called a Distinguished Name or a DN.
     There are quite a few fields but you can leave some blank
     For some fields there will be a default value,
     If you enter '.', the field will be left blank.
     -----
     Country Name (2 letter code) [AU]:
     State or Province Name (full name) [Some-State]:Queensland
     Locality Name (eg, city) []:Brisbane
     Organization Name (eg, company) [Internet Widgits Pty Ltd]:Docker Inc
     Organizational Unit Name (eg, section) []:Boot2Docker
     Common Name (e.g. server FQDN or YOUR name) []:your.host.com
     Email Address []:Sven@home.org.au

Now that we have a CA, you can create a server key and certificate
signing request (CSR). Make sure that "Common Name" (i.e. server FQDN or YOUR
name) matches the hostname you will use to connect to Docker:

    $ openssl genrsa -out server-key.pem 2048
    Generating RSA private key, 2048 bit long modulus
    ......................................................+++
    ............................................+++
    e is 65537 (0x10001)
    $ openssl req -subj '/CN=<Your Hostname Here>' -new -key server-key.pem -out server.csr

Next, we're going to sign the key with our CA:

    $ openssl x509 -req -days 365 -in server.csr -CA ca.pem -CAkey ca-key.pem \
      -CAcreateserial -out server-cert.pem
    Signature ok
    subject=/CN=your.host.com
    Getting CA Private Key
    Enter pass phrase for ca-key.pem:

For client authentication, create a client key and certificate signing
request:

    $ openssl genrsa -out key.pem 2048
    Generating RSA private key, 2048 bit long modulus
    ...............................................+++
    ...............................................................+++
    e is 65537 (0x10001)
    $ openssl req -subj '/CN=client' -new -key key.pem -out client.csr

To make the key suitable for client authentication, create an extensions
config file:

    $ echo extendedKeyUsage = clientAuth > extfile.cnf

Now sign the key:

    $ openssl x509 -req -days 365 -in client.csr -CA ca.pem -CAkey ca-key.pem \
      -CAcreateserial -out cert.pem -extfile extfile.cnf
    Signature ok
    subject=/CN=client
    Getting CA Private Key
    Enter pass phrase for ca-key.pem:

Now you can make the Docker daemon only accept connections from clients
providing a certificate trusted by our CA:

    $ docker -d --auth=cert --auth-ca=ca.pem --auth-cert=server-cert.pem --auth-key=server-key.pem \
      -H=0.0.0.0:2376

To be able to connect to Docker and validate its certificate, you now
need to provide your client keys, certificates and trusted CA:

    $ docker --auth=cert --auth-ca=ca.pem --auth-cert=cert.pem --auth-key=key.pem \
      -H=dns-name-of-docker-host:2376 version

> **Note**:
> Docker over TLS should run on TCP port 2376.

> **Warning**:
> As shown in the example above, you don't have to run the `docker` client
> with `sudo` or the `docker` group when you use certificate authentication.
> That means anyone with the keys can give any instructions to your Docker
> daemon, giving them root access to the machine hosting the daemon. Guard
> these keys as you would a root password!

## Secure by default

If you want to secure your Docker client connections by default, you can move
the files to the `.docker` directory in your home directory - and set the
`DOCKER_HOST` and `DOCKER_TLS_VERIFY` variables as well (instead of passing
`-H=tcp://:2376` and `--tlsverify` on every call).

    $ cp ca.pem ~/.docker/ca.pem
    $ cp cert.pem ~/.docker/cert.pem
    $ cp key.pem ~/.docker/key.pem
    $ export DOCKER_HOST=tcp://:2376
    $ export DOCKER_TLS_VERIFY=1

Docker will now connect securely by default:

    $ docker ps

### Other certificate-based modes

If you don't want to have complete two-way authentication, you can run
Docker in various other modes by mixing the flags.

#### Daemon modes

 - `tlsverify`, `tlscacert`, `tlscert`, `tlskey` set: Authenticate clients
 - `tls`, `tlscert`, `tlskey`: Do not authenticate clients

#### Client modes

 - `tls`: Authenticate server based on public/default CA pool
 - `tlsverify`, `tlscacert`: Authenticate server based on given CA
 - `tls`, `tlscert`, `tlskey`: Authenticate with client certificate, do not
   authenticate server based on given CA
 - `tlsverify`, `tlscacert`, `tlscert`, `tlskey`: Authenticate with client
   certificate and authenticate server based on given CA

#### Automatic configuration

If found, the client will send its client certificate, so you just need
to drop your keys into `~/.docker/<ca, cert or key>.pem`. Alternatively,
if you want to store your keys in another location, you can specify that
location using the environment variable `DOCKER_CERT_PATH`.

    $ export DOCKER_CERT_PATH=${HOME}/.docker/zone1/
    $ docker --auth=cert ps

### Connecting to the Secure Docker port using `curl`

To use `curl` to make test API requests, you need to use three extra command
line flags:

    $ curl https://boot2docker:2376/images/json \
      --cert ~/.docker/cert.pem \
      --key ~/.docker/key.pem \
      --cacert ~/.docker/ca.pem
