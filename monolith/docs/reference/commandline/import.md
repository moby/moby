---
title: "import"
description: "The import command description and usage"
keywords: "import, file, system, container"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# import

```markdown
Usage:  docker import [OPTIONS] file|URL|- [REPOSITORY[:TAG]]

Import the contents from a tarball to create a filesystem image

Options:
  -c, --change value     Apply Dockerfile instruction to the created image (default [])
      --help             Print usage
  -m, --message string   Set commit message for imported image
```

## Description

You can specify a `URL` or `-` (dash) to take data directly from `STDIN`. The
`URL` can point to an archive (.tar, .tar.gz, .tgz, .bzip, .tar.xz, or .txz)
containing a filesystem or to an individual file on the Docker host.  If you
specify an archive, Docker untars it in the container relative to the `/`
(root). If you specify an individual file, you must specify the full path within
the host. To import from a remote location, specify a `URI` that begins with the
`http://` or `https://` protocol.

The `--change` option will apply `Dockerfile` instructions to the image
that is created.
Supported `Dockerfile` instructions:
`CMD`|`ENTRYPOINT`|`ENV`|`EXPOSE`|`ONBUILD`|`USER`|`VOLUME`|`WORKDIR`

## Examples

### Import from a remote location

This will create a new untagged image.

```bash
$ docker import http://example.com/exampleimage.tgz
```

### Import from a local file

- Import to docker via pipe and `STDIN`.

  ```bash
  $ cat exampleimage.tgz | docker import - exampleimagelocal:new
  ```

- Import with a commit message.

  ```bash
  $ cat exampleimage.tgz | docker import --message "New image imported from tarball" - exampleimagelocal:new
  ```

- Import to docker from a local archive.

  ```bash
    $ docker import /path/to/exampleimage.tgz
  ```

### Import from a local directory

```bash
$ sudo tar -c . | docker import - exampleimagedir
```

### Import from a local directory with new configurations

```bash
$ sudo tar -c . | docker import --change "ENV DEBUG true" - exampleimagedir
```

Note the `sudo` in this example – you must preserve
the ownership of the files (especially root ownership) during the
archiving with tar. If you are not root (or the sudo command) when you
tar, then the ownerships might not get preserved.
