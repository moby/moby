# idmapfs
A fuse filesystem which maps filesystem access based on uid/gid map
The purpose of idmapfs is specifically for mapping a filesystem tree (or a file I suppose) to a user namespace
where a (or a set of) UID's and/or GID's are mapped to a different set of UID's and/or GID's in the user namespace.  
A common example is to map an unprivileged user, e.g. UID `10000`, to UID `0` in the user namespace, thus giving a
user root-like privileges in the user namespace but really it's mapped to an unprivileged user.

## Why?

By definition, a user namespaces (setup a particular way) makes it so that the user thinks it is accessing things
as one user, but really it is another. This extends to file system access. As an example, `/etc/shadow` is typically
only accessible by the root user. In a user namespace the user may appear to be the root user but will not have access
to `/etc/shadow` because the real user ID is mapped to a non-root user. This is important for security isolation.

In some cases you may want to allow the user in a user namespace to access files as if they really are the user they think
they are. This is not currently possible with anything available in the kernel and as such you'd have to result to chown/chmod
to allow the user in the user namespace the proper access, which is generally undesirable. `idmapfs` enables this functionality
through fuse.

*Note*: It is important to understand that the intention of idmapfs is to allow an administrator to pole a hole in the security
that user namespaces provides by giving user(s) in the user namespace access to files they would not normally.

## Build

```
go build ./cmd/idmapfs
```

## Usage

Map UID/GID 0 (and only 0) to UID/GID 10000.

```
./idmapfs --map-uids 0:10000:1 --map-gids 0:10000:1 <source> <target>
```

In the map-uids/map-gids spec, the notation is `<id to map from>:<mapped id range start>:<number of ids to map>`

Map UID/GID's 0-1000 to 10000-11000:

```
./idmapfs --map-uids 0:10000:1000 --map-gids 0:10000:1000 <source> <target>
```

If there is a UID/GID in `<source>` that is not mapped, it will retain it's original ownership.

## Other projects

[bindfs](https://bindfs.org) is another project which has some similar functionality, however the scope of it is much greater
and it only supports mapping a single UID/GID. `idmapfs` is specifically targeted at Linux user namespaces and even uses the
same idmapping syntax (although it does work on MacOS). `idmapfs` does not and will not support other features that bindfs does like
changing file permissions at mount time (chown/chmod are supported, though).

## Performance

No idea yet... probably slow. There is room for optimization.

## Status

This is very new and should be considered pre-alpha. It should not be considered secure or stable.
