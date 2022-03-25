# Custom Root CA Certificates for Dev Containers

Any files with a `.crt` extension in this directory are added as trusted root
CA certificates in the `base` container. This is helpful if you are building
Moby in an environment that intercepts HTTPS traffic (like some corporate
networks) and are seeing TLS errors with the default settings.

Git ignores all `.crt` files in this directory to reduce the risk of committing
private certificates.
