<!--[metadata]>
+++
title = "Content trust in Docker"
description = "Enabling content trust in Docker"
keywords = ["content, trust, security, docker,  documentation"]
[menu.main]
parent= "smn_content_trust"
weight=-1
+++
<![end-metadata]-->

# Content trust in Docker

When transferring data among networked systems, *trust* is a central concern. In
particular, when communicating over an untrusted medium such as the internet, it
is critical to ensure the integrity and publisher of all the data a system
operates on. You use Docker to push and pull images (data) to a registry. Content trust
gives you the ability to both verify the integrity and the publisher of all the
data received from a registry over any channel.

Content trust is currently only available for users of the public Docker Hub. It
is currently not available for the Docker Trusted Registry or for private
registries.

## Understand trust in Docker

Content trust allows operations with a remote Docker registry to enforce
client-side signing and verification of image tags. Content trust provides the
ability to use digital signatures for data sent to and received from remote
Docker registries. These signatures allow client-side verification of the
integrity and publisher of specific image tags.

Currently, content trust is disabled by default. You must enabled it by setting
the `DOCKER_CONTENT_TRUST` environment variable.

Once content trust is enabled, image publishers can sign their images. Image consumers can
ensure that the images they use are signed. publishers and consumers can be
individuals alone or in organizations. Docker's content trust supports users and
automated processes such as builds.

### Image tags and content trust

An individual image record has the following identifier:

```
[REGISTRY_HOST[:REGISTRY_PORT]/]REPOSITORY[:TAG]
```

A particular image `REPOSITORY` can have multiple tags. For example, `latest` and
 `3.1.2` are both tags on the `mongo` image. An image publisher can build an image
 and tag combination many times changing the image with each build.

Content trust is associated with the `TAG` portion of an image. Each image
repository has a set of keys that image publishers use to sign an image tag.
Image publishers have discretion on which tags they sign.

An image repository can contain an image with one tag that is signed and another
tag that is not. For example, consider [the Mongo image
repository](https://hub.docker.com/r/library/mongo/tags/). The `latest`
tag could be unsigned while the `3.1.6` tag could be signed. It is the
responsibility of the image publisher to decide if an image tag is signed or
not. In this representation, some image tags are signed, others are not:

![Signed tags](images/tag_signing.png)

Publishers can choose to sign a specific tag or not. As a result, the content of
an unsigned tag and that of a signed tag with the same name may not match. For
example, a publisher can push a tagged image `someimage:latest` and sign it.
Later, the same publisher can push an unsigned `someimage:latest` image. This second
push replaces the last unsigned tag `latest` but does not affect the signed `latest` version.
The ability to choose which tags they can sign, allows publishers to iterate over
the unsigned version of an image before officially signing it.

Image consumers can enable content trust to ensure that images they use were
signed. If a consumer enables content trust, they can only pull, run, or build
with trusted images. Enabling content trust is like wearing a pair of
rose-colored glasses. Consumers "see" only signed images tags and the less
desirable, unsigned image tags are "invisible" to them.

![Trust view](images/trust_view.png)

To the consumer who does not enabled content trust, nothing about how they
work with Docker images changes. Every image is visible regardless of whether it
is signed or not.


### Content trust operations and keys

When content trust is enabled, `docker` CLI commands that operate on tagged images must
either have content signatures or explicit content hashes. The commands that
operate with content trust are:

* `push`
* `build`
* `create`
* `pull`
* `run`

For example, with content trust enabled a `docker pull someimage:latest` only
succeeds if `someimage:latest` is signed. However, an operation with an explicit
content hash always succeeds as long as the hash exists:

```bash
$ docker pull someimage@sha256:d149ab53f8718e987c3a3024bb8aa0e2caadf6c0328f1d9d850b2a2a67f2819a
```

Trust for an image tag is managed through the use of signing keys. A key set is
created when an operation using content trust is first invoked. Docker's content
trust makes use of four different keys:

| Key                 | Description                                                                                                                                                                                                                                                                                                                                                                         |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| root key         | Root of content trust for a image tag. When content trust is enabled, you create the root key once. |
| target and snapshot | These two keys are known together as the "repository" key. When content trust is enabled, you create this key when you add a new image repository. If you have the root key, you can export the repository key and allow other publishers to sign the image tags.    |
| timestamp           | This key applies to a repository. It allows Docker repositories to have freshness security guarantees without requiring periodic content refreshes on the client's side.                                                                                                              |

With the exception of the timestamp, all the keys are generated and stored locally
client-side. The timestamp is safely generated and stored in a signing server that
is deployed alongside the Docker registry. All keys are generated in a backend
service that isn't directly exposed to the internet and are encrypted at rest.

The following image depicts the various signing keys and their relationships:

![Content trust components](images/trust_components.png)

>**WARNING**: Loss of the root key is **very difficult** to recover from.
>Correcting this loss requires intervention from [Docker
>Support](https://support.docker.com) to reset the repository state. This loss
>also requires **manual intervention** from every consumer that used a signed
>tag from this repository prior to the loss.

You should backup the root key somewhere safe. Given that it is only required
to create new repositories, it is a good idea to store it offline. Make sure you
read [Manage keys for content trust](trust_key_mng.md) information
for details on securing, and backing up your keys. 

## Survey of typical content trust operations

This section surveys the typical trusted operations users perform with Docker
images.

### Enable and disable content trust per-shell or per-invocation

In a shell, you can enable content trust by setting the `DOCKER_CONTENT_TRUST`
environment variable. Enabling per-shell is useful because you can have one
shell configured for trusted operations and another terminal shell for untrusted
operations. You can also add this declaration to your shell profile to have it
turned on always by default.

To enable content trust in a `bash` shell enter the following command:

```bash
export DOCKER_CONTENT_TRUST=1
```

Once set, each of the "tag" operations requires a key for a trusted tag.

In an environment where `DOCKER_CONTENT_TRUST` is set, you can use the
`--disable-content-trust` flag to run individual operations on tagged images
without content trust on an as-needed basis.

```bash
$  docker pull --disable-content-trust docker/trusttest:untrusted
```

To invoke a command with content trust enabled regardless of whether or how the `DOCKER_CONTENT_TRUST` variable is set:

```bash
$  docker build --disable-content-trust=false -t docker/trusttest:testing .
```

All of the trusted operations support the `--disable-content-trust` flag.


### Push trusted content

To create signed content for a specific image tag, simply enable content trust
and push a tagged image. If this is the first time you have pushed an image
using content trust on your system, the session looks like this:

```bash
$ docker push docker/trusttest:latest
The push refers to a repository [docker.io/docker/trusttest] (len: 1)
9a61b6b1315e: Image already exists
902b87aaaec9: Image already exists
latest: digest: sha256:d02adacee0ac7a5be140adb94fa1dae64f4e71a68696e7f8e7cbf9db8dd49418 size: 3220
Signing and pushing trust metadata
You are about to create a new root signing key passphrase. This passphrase
will be used to protect the most sensitive key in your signing system. Please
choose a long, complex passphrase and be careful to keep the password and the
key file itself secure and backed up. It is highly recommended that you use a
password manager to generate the passphrase and keep it safe. There will be no
way to recover this key. You can find the key in your config directory.
Enter passphrase for new root key with id a1d96fb:
Repeat passphrase for new root key with id a1d96fb:
Enter passphrase for new repository key with id docker.io/docker/trusttest (3a932f1):
Repeat passphrase for new repository key with id docker.io/docker/trusttest (3a932f1):
Finished initializing "docker.io/docker/trusttest"
```
When you push your first tagged image with content trust enabled, the  `docker`
client recognizes this is your first push and:

 - alerts you that it will create a new root key
 - requests a passphrase for the key
 - generates a root key in the `~/.docker/trust` directory
 - generates a repository key for in the `~/.docker/trust` directory

The passphrase you chose for both the root key and your content key-pair
should be randomly generated and stored in a *password manager*.

> **NOTE**: If you omit the `latest` tag, content trust is skipped. This is true
even if content trust is enabled and even if this is your first push.

```bash
$ docker push docker/trusttest
The push refers to a repository [docker.io/docker/trusttest] (len: 1)
9a61b6b1315e: Image successfully pushed
902b87aaaec9: Image successfully pushed
latest: digest: sha256:a9a9c4402604b703bed1c847f6d85faac97686e48c579bd9c3b0fa6694a398fc size: 3220
No tag specified, skipping trust metadata push
```

It is skipped because as the message states, you did not supply an image `TAG`
value. In Docker content trust, signatures are associated with tags.

Once you have a root key on your system, subsequent images repositories
you create can use that same root key:

```bash
$ docker push docker.io/docker/seaside:latest
The push refers to a repository [docker.io/docker/seaside] (len: 1)
a9539b34a6ab: Image successfully pushed
b3dbab3810fc: Image successfully pushed
latest: digest: sha256:d2ba1e603661a59940bfad7072eba698b79a8b20ccbb4e3bfb6f9e367ea43939 size: 3346
Signing and pushing trust metadata
Enter key passphrase for root key with id a1d96fb:
Enter passphrase for new repository key with id docker.io/docker/seaside (bb045e3):
Repeat passphrase for new repository key with id docker.io/docker/seaside (bb045e3):
Finished initializing "docker.io/docker/seaside"
```

The new image has its own repository key and timestamp key. The `latest` tag is signed with both of
these.


### Pull image content

A common way to consume an image is to `pull` it. With content trust enabled, the Docker
client only allows `docker pull` to retrieve signed images.

```
$  docker pull docker/seaside
Using default tag: latest
Pull (1 of 1): docker/trusttest:latest@sha256:d149ab53f871
...
Tagging docker/trusttest@sha256:d149ab53f871 as docker/trusttest:latest
```

The `seaside:latest` image is signed. In the following example, the command does not specify a tag, so the system uses
the `latest` tag by default again and the `docker/cliffs:latest` tag is not signed.

```bash
$ docker pull docker/cliffs
Using default tag: latest
no trust data available
```

Because the tag `docker/cliffs:latest` is not trusted, the `pull` fails.


### Disable content trust for specific operations

A user that wants to disable content trust for a particular operation can use the
`--disable-content-trust` flag. **Warning: this flag disables content trust for
this operation**. With this flag, Docker will ignore content-trust and allow all
operations to be done without verifying any signatures. If we wanted the
previous untrusted build to succeed we could do:

```
$  cat Dockerfile
FROM docker/trusttest:notrust
RUN echo
$  docker build --disable-content-trust -t docker/trusttest:testing .
Sending build context to Docker daemon 42.84 MB
...
Successfully built f21b872447dc
```

The same is true for all the other commands, such as `pull` and `push`:

```
$  docker pull --disable-content-trust docker/trusttest:untrusted
...
$  docker push --disable-content-trust docker/trusttest:untrusted
...
```

## Related information

* [Manage keys for content trust](trust_key_mng.md)
* [Automation with content trust](trust_automation.md)
* [Play in a content trust sandbox](trust_sandbox.md)
