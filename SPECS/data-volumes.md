
## Spec for data volumes

Spec owner: Solomon Hykes <solomon@dotcloud.com>

Data volumes (issue #111) are a much-requested feature which trigger much discussion and debate. Below is the current authoritative spec for implementing data volumes.
This spec will be deprecated once the feature is fully implemented.

Discussion, requests, trolls, demands, offerings, threats and other forms of supplications concerning this spec should be addressed to Solomon here: https://github.com/dotcloud/docker/issues/111


### 1. Creating data volumes

At container creation, parts of a container's filesystem can be mounted as separate data volumes. Volumes are defined with the -v flag.

For example:

```bash
$ docker run -v /var/lib/postgres -v /var/log postgres /usr/bin/postgres
```

In this example, a new container is created from the 'postgres' image. At the same time, docker creates 2 new data volumes: one will be mapped to the container at /var/lib/postgres, the other at /var/log.

2 important notes:

1) Volumes don't have top-level names. At no point does the user provide a name, or is a name given to him. Volumes are identified by the path at which they are mounted inside their container.

2) The user doesn't choose the source of the volume. Docker only mounts volumes it created itself, in the same way that it only runs containers that it created itself. That is by design.


### 2. Sharing data volumes

Instead of creating its own volumes, a container can share another container's volumes. For example:

```bash
$ docker run --volumes-from $OTHER_CONTAINER_ID postgres /usr/local/bin/postgres-backup
```

In this example, a new container is created from the 'postgres' example. At the same time, docker will *re-use* the 2 data volumes created in the previous example. One volume will be mounted on the /var/lib/postgres of *both* containers, and the other will be mounted on the /var/log of both containers.

### 3. Under the hood

Docker stores volumes in /var/lib/docker/volumes. Each volume receives a globally unique ID at creation, and is stored at /var/lib/docker/volumes/ID.

At creation, volumes are attached to a single container - the source of truth for this mapping will be the container's configuration.

Mounting a volume consists of calling "mount --bind" from the volume's directory to the appropriate sub-directory of the container mountpoint. This may be done by Docker itself, or farmed out to lxc (which supports mount-binding) if possible.


### 4. Backups, transfers and other volume operations

Volumes sometimes need to be backed up, transfered between hosts, synchronized, etc. These operations typically are application-specific or site-specific, eg. rsync vs. S3 upload vs. replication vs...

Rather than attempting to implement all these scenarios directly, Docker will allow for custom implementations using an extension mechanism.

### 5. Custom volume handlers

Docker allows for arbitrary code to be executed against a container's volumes, to implement any custom action: backup, transfer, synchronization across hosts, etc.

Here's an example:

```bash
$ DB=$(docker run -d -v /var/lib/postgres -v /var/log postgres /usr/bin/postgres)

$ BACKUP_JOB=$(docker run -d --volumes-from $DB shykes/backuper /usr/local/bin/backup-postgres --s3creds=$S3CREDS)

$ docker wait $BACKUP_JOB
```

Congratulations, you just implemented a custom volume handler, using Docker's built-in ability to 1) execute arbitrary code and 2) share volumes between containers.

