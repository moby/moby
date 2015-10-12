<!--[metadata]>
+++
title = "Manage keys for content trust"
description = "Manage keys for content trust"
keywords = ["trust, security, root,  keys, repository"]
[menu.main]
parent= "smn_content_trust"
+++
<![end-metadata]-->

# Manage keys for content trust

Trust for an image tag is managed through the use of keys. Docker's content
trust makes use four different keys:

| Key                 | Description                                                                                                                                                                                                                                                                                                                                                                         |
|---------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| root key         | Root of content trust for a image tag. When content trust is enabled, you create the root key once. |
| target and snapshot | These two keys are known together as the "repository" key. When content trust is enabled, you create this key when you add a new image repository. If you have the root key, you can export the repository key and allow other publishers to sign the image tags.    |
| timestamp           | This key applies to a repository. It allows Docker repositories to have freshness security guarantees without requiring periodic content refreshes on the client's side.                                                                                                              |

With the exception of the timestamp, all the keys are generated and stored locally
client-side. The timestamp is safely generated and stored in a signing server that
is deployed alongside the Docker registry. All keys are generated in a backend
service that isn't directly exposed to the internet and are encrypted at rest.

## Choosing a passphrase

The passphrases you chose for both the root key and your repository key should
be randomly generated and stored in a password manager.  Having the repository key
allow users to sign image tags on a repository. Passphrases are used to encrypt
your keys at rest and ensures that a lost laptop or an unintended backup doesn't
put the private key material at risk.

## Back up your keys

All the Docker trust keys are stored encrypted using the passphrase you provide
on creation. Even so, you should still take care of the location where you back them up.
Good practice is to create two encrypted USB keys.

It is very important that you backup your keys to a safe, secure location. Loss
of the repository key is recoverable; loss of the root key is not.

The Docker client stores the keys in the `~/.docker/trust/private` directory.
Before backing them up, you should `tar` them into an archive:

```bash
$ umask 077; tar -zcvf private_keys_backup.tar.gz ~/.docker/trust/private; umask 022
```

## Lost keys

If a publisher loses keys it means losing the ability to sign trusted content for
your repositories.  If you lose a key, contact [Docker
Support](https://support.docker.com) (support@docker.com) to reset the repository
state.

This loss also requires **manual intervention** from every consumer that pulled
the tagged image prior to the loss. Image consumers would get an error for
content that they already downloaded:

```
could not validate the path to a trusted root: failed to validate data with current trusted certificates
```

To correct this, they need to download a new image tag with that is signed with
the new key.

## Related information

* [Content trust in Docker](content_trust.md) 
* [Automation with content trust](trust_automation.md)
* [Play in a content trust sandbox](trust_sandbox.md)
