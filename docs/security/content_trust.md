<!--[metadata]>
+++
alias = [TBD]
title = "Content trust in Docker"
description = "Enabling content trust in Docker"
keywords = ["trust, security, docker,  documentation"]
[menu.main]
parent= "smn_administrate"
+++
<![end-metadata]-->

## Enabling content trust in Docker

Docker 1.8 enables content trust. Content trust means that all operations with a remote registry enforce  client-side signing and verification of content tags. Currently content trust is opt-in. We are planning to change this behavior to opt-out in a future release. For now, in order to have trusted operations with Docker you will need to either `export DOCKER_CONTENT_TRUST=1` , or use the `--disable-content-trust=false` flag.

When enabled, content trust will ensure that all operations using a remote registry enforce the signing and verification of tags. In particular `push`, `pull`, `build`, `create` and `run` will only operate on tags that either have content signatures or explicit content hashes.

#### Pulling trusted content
Let's try to pull a remote repository on the Docker hub that does not have content-trust data:

```
➜ export DOCKER_CONTENT_TRUST=1
➜ docker pull diogomonica/alpine
Using default tag: latest
no trust data available
```

As we can see, since we have docker content trust enabled, we are not allowed to pull from a repository that does not have signed content.

#### Pushing trusted content

To create signed content for a specific tag you simply have to push to a repository, providing a content tag. However, if this is the first time you are pushing to a remote repository with content trust enabled, two things will happen:

 - A new root key will be generated and you will be asked to provide a phassphrase for this key
 - A new tagging pair will be generated, and you will also be asked to provide a passphrase for these keys

```
# docker push diogomonica/alpine:latest
...
You are about to create a new root signing key passphrase.
You can find the key in your config directory.
Enter passphrase for new root key with id 3a97f:
Repeat passphrase for new root key with id 3a97f:
Enter passphrase for new targets key with id docker.io/diogomonica/alpine/03cf0:
Repeat passphrase for new targets key with id docker.io/diogomonica/alpine/03cf0:
Finished initializing "docker.io/diogomonica/alpine"
```

It is **very important** to note that loss of the root key will require contacting Docker's support team to reset the repository state and will also require **manual intervention** from every docker user that has used the repository before (removing the old root certificate). You should backup this key somewhere safe. We will show you how to backup your keys later on in this document.

Additionally, the passphrase you chose for both the root key and your content key-pair should be randomly generated and stored in a **password manager**.  You can find more details on how these keys are used in the content trust key structure section.

Now that we have pushed the tag `latest`, we can confirm that our trusted pull now works as expected:

```
➜  docker pull diogomonica/alpine
Using default tag: latest
Pull (1 of 1): diogomonica/alpine:latest@sha256:d149ab53f871
...
Tagging diogomonica/alpine@sha256:d149ab53f871 as diogomonica/alpine:latest
```

#### Building with trusted content

Now lets try to build the simple Dockerfile below. Since we are using `FROM diogomonica/alpine:latest`, we are expecting this build to succeed, given that trust data exists for the tag `latest`.

```
➜  cat Dockerfile
FROM diogomonica/alpine:latest
RUN echo
➜  docker build -t diogomonica/alpine:testing .
Using default tag: latest
latest: Pulling from diogomonica/alpine

b3dbab3810fc: Pull complete
a9539b34a6ab: Pull complete
Digest: sha256:d149ab53f871
```

If we try to build from a Dockerfile that relied on tag without trust data, the build command will fail:

```
➜  cat Dockerfile
FROM diogomonica/alpine:notrust
RUN echo
➜  docker build -t diogomonica/alpine:testing .
unable to process Dockerfile: No trust data for notrust
```

#### Doing untrusted operations

A user that wants to disable trust for a particular operation can use the `--disable-content-trust` flag. **Warning: this flag disables content trust for this operation**. With this flag, Docker will ignore content-trust and allow all operations to be done without verifying any signatures. If we wanted the previous untrusted build to succeed we could do:

```
➜  cat Dockerfile
FROM diogomonica/alpine:notrust
RUN echo
➜  docker build --disable-content-trust -t diogomonica/alpine:testing .
Sending build context to Docker daemon 42.84 MB
...
Successfully built f21b872447dc
```

The same is true for all the other commands, such as `pull` and `push`:

```
➜  docker pull --disable-content-trust diogomonica/alpine:untrusted
...
➜  docker push --disable-content-trust diogomonica/alpine:untrusted
...
```

### Backing up your keys

Losing your keys means losing the ability to sign trusted content for your repositories. It is very important that you backup these keys to a safe, secure location. Loss of the tagging pair is recoverable; loss of the root key is not.

While all the keys are stored encrypted at rest with the passphrase you provide on creation, you should still take care of the location where you back them up. Two encrypted USB keys should do the trick.

All the private keys used with content trust are stored in `~/.docker/trust/private`. You can back them up by doing:
```
➜ tar -zcvf private_keys_backup.tar.gz ~/.docker/trust/private
➜ chmod 600 private_keys_backup.tar.gz
```


----------

### Content trust key structure
Docker's content trust makes use of four different keys: root, snapshots, targets and timestamps. Only the first three are generated and stored client-side.

 - Root: the root key is the most important key, since it is the root of trust for your repository. Different repositories can use the same root key. You will only need this key if you are creating a new repository or rotating an existing key.
 - Target/Snapshot: we call these two keys the tagging pair. A new pair is generated for each new repository you own. They can be exported and shared with any person/system that needs to be able to sign content for this repository.
 - Timestamp: this key is generated and stored on Docker's servers, and allows Docker repositories to have freshness security guarantees without the hassle of having to constantly refresh the content client-side.

----------

### Automation

In order to allow tools to wrap docker and push trusted content, there are three environment variables that allow you to provide the passphrases without an expect script, or typing them in:

 - DOCKER_CONTENT_TRUST_ROOT_PASSPHRASE
 - DOCKER_CONTENT_TRUST_TARGET_PASSPHRASE
 - DOCKER_CONTENT_TRUST_SNAPSHOT_PASSPHRASE

Docker will attempt to use the contents of these environment variables as passphrase for the keys. For example, if I export both the `target` and `snapshot` passphrases, I won't be asked for them when pushing a new tag:

```
➜  export DOCKER_CONTENT_TRUST_TARGET_PASSPHRASE="u7pEQcGoebUHm6LHe6"
➜  export DOCKER_CONTENT_TRUST_SNAPSHOT_PASSPHRASE="u7pEQcGoebUHm6LHe6"
➜  docker push diogomonica/alpine:latest
The push refers to a repository [docker.io/diogomonica/alpine] (len: 1)
a9539b34a6ab: Image already exists
b3dbab3810fc: Image already exists
latest: digest: sha256:d149ab53f871 size: 3355
Signing and pushing trust metadata
```