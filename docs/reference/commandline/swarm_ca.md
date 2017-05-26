---
title: "swarm ca"
description: "The swarm ca command description and usage"
keywords: "swarm, ca"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# swarm ca

```markdown
Usage:	docker swarm ca [OPTIONS]

Manage root CA

Options:
      --ca-cert pem-file          Path to the PEM-formatted root CA certificate to use for the new cluster
      --ca-key pem-file           Path to the PEM-formatted root CA key to use for the new cluster
      --cert-expiry duration      Validity period for node certificates (ns|us|ms|s|m|h) (default 2160h0m0s)
  -d, --detach                    Exit immediately instead of waiting for the root rotation to converge
      --external-ca external-ca   Specifications of one or more certificate signing endpoints
      --help                      Print usage
  -q, --quiet                     Suppress progress output
      --rotate                    Rotate the swarm CA - if no certificate or key are provided, new ones will be generated
```

## Description

View or rotate the current swarm CA certificate. This command must target a manager node.

## Examples

Run the `docker swarm ca` command without any options to view the current root CA certificate
in PEM format.

```bash
$ docker swarm ca
-----BEGIN CERTIFICATE-----
MIIBazCCARCgAwIBAgIUJPzo67QC7g8Ebg2ansjkZ8CbmaswCgYIKoZIzj0EAwIw
EzERMA8GA1UEAxMIc3dhcm0tY2EwHhcNMTcwNTAzMTcxMDAwWhcNMzcwNDI4MTcx
MDAwWjATMREwDwYDVQQDEwhzd2FybS1jYTBZMBMGByqGSM49AgEGCCqGSM49AwEH
A0IABKL6/C0sihYEb935wVPRA8MqzPLn3jzou0OJRXHsCLcVExigrMdgmLCC+Va4
+sJ+SLVO1eQbvLHH8uuDdF/QOU6jQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMB
Af8EBTADAQH/MB0GA1UdDgQWBBSfUy5bjUnBAx/B0GkOBKp91XvxzjAKBggqhkjO
PQQDAgNJADBGAiEAnbvh0puOS5R/qvy1PMHY1iksYKh2acsGLtL/jAIvO4ACIQCi
lIwQqLkJ48SQqCjG1DBTSBsHmMSRT+6mE2My+Z3GKA==
-----END CERTIFICATE-----
```

Pass the `--rotate` flag (and optionally a `--ca-cert`, along with a `--ca-key` or
`--external-ca` parameter flag), in order to rotate the current swarm root CA.

```
$ docker swarm ca --rotate
desired root digest: sha256:05da740cf2577a25224c53019e2cce99bcc5ba09664ad6bb2a9425d9ebd1b53e
  rotated TLS certificates:  [=========================>                         ] 1/2 nodes
  rotated CA certificates:   [>                                                  ] 0/2 nodes
```

Once the rotation os finished (all the progress bars have completed) the now-current
CA certificate will be printed:

```
$ docker swarm ca --rotate
desired root digest: sha256:05da740cf2577a25224c53019e2cce99bcc5ba09664ad6bb2a9425d9ebd1b53e
  rotated TLS certificates:  [==================================================>] 2/2 nodes
  rotated CA certificates:   [==================================================>] 2/2 nodes
-----BEGIN CERTIFICATE-----
MIIBazCCARCgAwIBAgIUFynG04h5Rrl4lKyA4/E65tYKg8IwCgYIKoZIzj0EAwIw
EzERMA8GA1UEAxMIc3dhcm0tY2EwHhcNMTcwNTE2MDAxMDAwWhcNMzcwNTExMDAx
MDAwWjATMREwDwYDVQQDEwhzd2FybS1jYTBZMBMGByqGSM49AgEGCCqGSM49AwEH
A0IABC2DuNrIETP7C7lfiEPk39tWaaU0I2RumUP4fX4+3m+87j0DU0CsemUaaOG6
+PxHhGu2VXQ4c9pctPHgf7vWeVajQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMB
Af8EBTADAQH/MB0GA1UdDgQWBBSEL02z6mCI3SmMDmITMr12qCRY2jAKBggqhkjO
PQQDAgNJADBGAiEA263Eb52+825EeNQZM0AME+aoH1319Zp9/J5ijILW+6ACIQCg
gyg5u9Iliel99l7SuMhNeLkrU7fXs+Of1nTyyM73ig==
-----END CERTIFICATE-----
```

### `--rotate`

Root CA Rotation is recommended if one or more of the swarm managers have been
compromised, so that those managers can no longer connect to or be trusted by
any other node in the cluster.

Alternately, root CA rotation can be used to give control of the swarm CA
to an external CA, or to take control back from an external CA.

The `--rotate` flag does not require any parameters to do a rotation, but you can
optionally specify a certificate and key, or a certificate and external CA URL,
and those will be used instead of an automatically-generated certificate/key pair.

Because the root CA key should be kept secret, if provided it will not be visible
when viewing swarm any information via the CLI or API.

The root CA rotation will not be completed until all registered nodes have
rotated their TLS certificates.  If the rotation is not completing within a
reasonable amount of time, try running
`docker node ls --format {{.ID}} {{.Hostname}} {{.Status}} {{.TLSStatus}}` to
see if any nodes are down or otherwise unable to rotate TLS certificates.


### `--detach`

Initiate the root CA rotation, but do not wait for the completion of or display the
progress of the rotation.

## Related commands

* [swarm init](swarm_init.md)
* [swarm join](swarm_join.md)
* [swarm join-token](swarm_join_token.md)
* [swarm leave](swarm_leave.md)
* [swarm unlock](swarm_unlock.md)
* [swarm unlock-key](swarm_unlock_key.md)
