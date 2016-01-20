<!-- [metadata]>
+++
title = "Work with Docker security"
description = "Sec"
keywords = ["seccomp, security, docker, documentation"]
[menu.main]
identifier="smn_secure_docker"
parent= "mn_use_docker"
+++
<![end-metadata]-->

# Work with Docker security

This section discusses the security features you can configure and use within your Docker Engine installation.

* You can configure Docker's trust features so that your users can push and pull trusted images. To learn how to do this, see [Use trusted images](trust/index.md) in this section.

* You can protect the Docker daemon socket and ensure only trusted Docker client connections. For more information, [Protect the Docker daemon socket](https.md)

* You can use certificate-based client-server authentication to verify a Docker daemon has the rights to access images on a registry. For more information, see [Using certificates for repository client verification](certificates.md).

* You can configure secure computing mode (Seccomp) policies to secure system calls in a container. For more information, see [Seccomp security profiles for Docker](seccomp.md).

* An AppArmor profile for Docker is installed with the official *.deb* packages. For information about this profile and overriding it, see [AppArmor security profiles for Docker](apparmor.md).
