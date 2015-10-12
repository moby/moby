<!--[metadata]>
+++
title = "Automation with content trust"
description = "Automating content push pulls with trust"
keywords = ["trust, security, docker,  documentation, automation"]
[menu.main]
parent= "smn_content_trust"
+++
<![end-metadata]-->

# Automation with content trust

Your automation systems that pull or build images can also work with trust. Any automation environment must set `DOCKER_TRUST_ENABLED` either manually or in in a scripted fashion before processing images.

## Bypass requests for passphrases

To allow tools to wrap docker and push trusted content, there are two
environment variables that allow you to provide the passphrases without an
expect script, or typing them in:

 - `DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE`
 - `DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE`

Docker attempts to use the contents of these environment variables as passphrase
for the keys. For example, an image publisher can export the repository `target`
and `snapshot` passphrases:

```bash
$  export DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE="u7pEQcGoebUHm6LHe6"
$  export DOCKER_CONTENT_TRUST_REPOSITORY_PASSPHRASE="l7pEQcTKJjUHm6Lpe4"
```

Then, when pushing a new tag the Docker client does not request these values but signs automatically:

```bash
$  docker push docker/trusttest:latest
The push refers to a repository [docker.io/docker/trusttest] (len: 1)
a9539b34a6ab: Image already exists
b3dbab3810fc: Image already exists
latest: digest: sha256:d149ab53f871 size: 3355
Signing and pushing trust metadata
```

## Building with content trust

You can also build with content trust. Before running the `docker build` command, you should set the environment variable `DOCKER_CONTENT_TRUST` either manually or in in a scripted fashion. Consider the simple Dockerfile below.

```Dockerfile
FROM docker/trusttest:latest
RUN echo
```

The `FROM` tag is pulling a signed image. You cannot build an image that has a
`FROM` that is not either present locally or signed. Given that content trust
data exists for the tag `latest`, the following build should succeed:

```bash
$  docker build -t docker/trusttest:testing .
Using default tag: latest
latest: Pulling from docker/trusttest

b3dbab3810fc: Pull complete
a9539b34a6ab: Pull complete
Digest: sha256:d149ab53f871
```

If content trust is enabled, building from a Dockerfile that relies on tag without trust data, causes the build command to fail:

```bash
$  docker build -t docker/trusttest:testing .
unable to process Dockerfile: No trust data for notrust
```

## Related information

* [Content trust in Docker](content_trust.md) 
* [Manage keys for content trust](trust_key_mng.md)
* [Play in a content trust sandbox](trust_sandbox.md)

