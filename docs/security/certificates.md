<!--[metadata]>
+++
aliases = ["/engine/articles/certificates/"]
title = "Using certificates for repository client verification"
description = "How to set up and use certificates with a registry to verify access"
keywords = ["Usage, registry, repository, client, root, certificate, docker, apache, ssl, tls, documentation, examples, articles,  tutorials"]
[menu.main]
parent = "smn_secure_docker"
+++
<![end-metadata]-->

# Using certificates for repository client verification

In [Running Docker with HTTPS](https.md), you learned that, by default,
Docker runs via a non-networked Unix socket and TLS must be enabled in order
to have the Docker client and the daemon communicate securely over HTTPS.  TLS ensures authenticity of the registry endpoint and that traffic to/from registry is encrypted.

This article demonstrates how to ensure the traffic between the Docker registry (i.e., *a server*) and the Docker daemon (i.e., *a client*) traffic is encrypted and a properly authenticated using *certificate-based client-server authentication*.

We will show you how to install a Certificate Authority (CA) root certificate
for the registry and how to set the client TLS certificate for verification.

## Understanding the configuration

A custom certificate is configured by creating a directory under
`/etc/docker/certs.d` using the same name as the registry's hostname (e.g.,
`localhost`). All `*.crt` files are added to this directory as CA roots.

> **Note:**
> In the absence of any root certificate authorities, Docker
> will use the system default (i.e., host's root CA set).

The presence of one or more `<filename>.key/cert` pairs indicates to Docker
that there are custom certificates required for access to the desired
repository.

> **Note:**
> If there are multiple certificates, each will be tried in alphabetical
> order. If there is an authentication error (e.g., 403, 404, 5xx, etc.), Docker
> will continue to try with the next certificate.

The following illustrates a configuration with multiple certs:

```
    /etc/docker/certs.d/        <-- Certificate directory
    └── localhost               <-- Hostname
       ├── client.cert          <-- Client certificate
       ├── client.key           <-- Client key
       └── localhost.crt        <-- Certificate authority that signed
                                    the registry certificate
```

The preceding example is operating-system specific and is for illustrative
purposes only. You should consult your operating system documentation for
creating an os-provided bundled certificate chain.


## Creating the client certificates

You will use OpenSSL's `genrsa` and `req` commands to first generate an RSA
key and then use the key to create the certificate.   

    $ openssl genrsa -out client.key 4096
    $ openssl req -new -x509 -text -key client.key -out client.cert

> **Note:**
> These TLS commands will only generate a working set of certificates on Linux.
> The version of OpenSSL in Mac OS X is incompatible with the type of
> certificate Docker requires.

## Troubleshooting tips

The Docker daemon interprets ``.crt` files as CA certificates and `.cert` files
as client certificates. If a CA certificate is accidentally given the extension
`.cert` instead of the correct `.crt` extension, the Docker daemon logs the
following error message:

```
Missing key KEY_NAME for client certificate CERT_NAME. Note that CA certificates should use the extension .crt.
```

## Related Information

* [Use trusted images](index.md)
* [Protect the Docker daemon socket](https.md)
