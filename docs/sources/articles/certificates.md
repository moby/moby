page_title: Using certificates for repository client verification
page_description: How to set up per-repository client certificates
page_keywords: Usage, repository, certificate, root, docker, documentation, examples

# Using certificates for repository client verification

This lets you specify custom client TLS certificates and CA root for a
specific registry hostname. Docker will then verify the registry
against the CA and present the client cert when talking to that
registry. This allows the registry to verify that the client has a
proper key, indicating that the client is allowed to access the
images.

A custom cert is configured by creating a directory in
`/etc/docker/certs.d` with the same name as the registry hostname. Inside
this directory all .crt files are added as CA Roots (if none exists,
the system default is used) and pair of files `$filename.key` and
`$filename.cert` indicate a custom certificate to present to the
registry.

If there are multiple certificates each one will be tried in
alphabetical order, proceeding to the next if we get a 403 of 5xx
response.

So, an example setup would be::

    /etc/docker/certs.d/
    └── localhost
       ├── client.cert
       ├── client.key
       └── localhost.crt

A simple way to test this setup is to use an apache server to host a
registry. Just copy a registry tree into the apache root,
[here](http://people.gnome.org/~alexl/v1.tar.gz) is an example one
containing the busybox image.

Then add this conf file as `/etc/httpd/conf.d/registry.conf`:

    # This must be in the root context, otherwise it causes a re-negotiation
    # which is not supported by the tls implementation in go
    SSLVerifyClient optional_no_ca

    <Location /v1>
    Action cert-protected /cgi-bin/cert.cgi
    SetHandler cert-protected

    Header set x-docker-registry-version "0.6.2"
    SetEnvIf Host (.*) custom_host=$1
    Header set X-Docker-Endpoints "%{custom_host}e"
    </Location>

And this as `/var/www/cgi-bin/cert.cgi`:

    #!/bin/bash
    if [ "$HTTPS" != "on" ]; then
        echo "Status: 403 Not using SSL"
        echo "x-docker-registry-version: 0.6.2"
        echo
        exit 0
    fi
    if [ "$SSL_CLIENT_VERIFY" == "NONE" ]; then
        echo "Status: 403 Client certificate invalid"
        echo "x-docker-registry-version: 0.6.2"
        echo
        exit 0
    fi
    echo "Content-length: $(stat --printf='%s' $PATH_TRANSLATED)"
    echo "x-docker-registry-version: 0.6.2"
    echo "X-Docker-Endpoints: $SERVER_NAME"
    echo "X-Docker-Size: 0"
    echo

    cat $PATH_TRANSLATED

This will return 403 for all accessed to `/v1` unless any client cert is
presented. Obviously a real implementation would verify more details
about the certificate.

Example client certs can be generated with::

    openssl genrsa -out client.key 1024
    openssl req -new -x509 -text -key client.key -out client.cert
