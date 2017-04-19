docker/hack
============================

## About

A collection of useful scripts that we use; we decided it deserves a great
README! If there are any issues regarding the intention of a particular script
(or even part of a certain script), please reach out to us. It may help us
either refine our current scripts, or add on new ones that are appropriate for
a given use case.

**Note** None of these files are intended for manual execution. Please refer to
https://docs.docker.com/opensource/project/test-and-docs/ which covers scripts
in this folder running via `make`

## DinD (dind.sh)

DinD is a wrapper script which allows Docker to be run inside a Docker
container. Please note that DinD should only be executed inside a Docker
container with privileged mode enabled

## Generate Authors (generate-authors.sh)

Generates AUTHORS; a file with all the names and corresponding emails of
individual Docker contributors. AUTHORS can be found in the home directory of
this repository.

## Install (install.sh)

Executable install script for installing Docker. If updates to this are
desired, please use hack/release.sh during a normal release. The following
one-liner may be used for script hotfixes:
- `aws s3 cp --acl public-read hack/install.sh s3://get.docker.com/index`

## Make

There are two make files, each with different extensions. Neither are supposed
to be called directly; only invoke `make`. Both scripts run inside a Docker
container.

### make.ps1

- The Windows native build script that uses PowerShell semantics; it is limited
unlike `hack\make.sh`.

### make.sh
- Builds a number of binary artifacts from a given version of the Docker source
code.

## Release (release.sh)

Releases any bundles built by `make` on a public AWS S3 bucket.

## Vendor (vendor.sh)

A shell script that is a wrapper around Vndr. For information on how to use
this, please refer to [Vndr's README].
(https://github.com/LK4D4/vndr/blob/master/README.md)
