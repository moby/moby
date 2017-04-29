---
title: "login"
description: "The login command description and usage"
keywords: "registry, login, image"
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

# login

```markdown
Usage:  docker login [OPTIONS] [SERVER]

Log in to a Docker registry.
If no server is specified, the default is defined by the daemon.

Options:
      --help              Print usage
  -p, --password string   Password
  -u, --username string   Username
```

## Description

Login to a registry.

### Login to a self-hosted registry

If you want to login to a self-hosted registry you can specify this by
adding the server name.

```bash
$ docker login localhost:8080
```

### Privileged user requirement

`docker login` requires user to use `sudo` or be `root`, except when:

1.  connecting to a remote daemon, such as a `docker-machine` provisioned `docker engine`.
2.  user is added to the `docker` group.  This will impact the security of your system; the `docker` group is `root` equivalent.  See [Docker Daemon Attack Surface](https://docs.docker.com/security/security/#docker-daemon-attack-surface) for details.

You can log into any public or private repository for which you have
credentials.  When you log in, the command stores encoded credentials in
`$HOME/.docker/config.json` on Linux or `%USERPROFILE%/.docker/config.json` on Windows.

### Credentials store

The Docker Engine can keep user credentials in an external credentials store,
such as the native keychain of the operating system. Using an external store
is more secure than storing credentials in the Docker configuration file.

To use a credentials store, you need an external helper program to interact
with a specific keychain or external store. Docker requires the helper
program to be in the client's host `$PATH`.

This is the list of currently available credentials helpers and where
you can download them from:

- D-Bus Secret Service: https://github.com/docker/docker-credential-helpers/releases
- Apple macOS keychain: https://github.com/docker/docker-credential-helpers/releases
- Microsoft Windows Credential Manager: https://github.com/docker/docker-credential-helpers/releases

You need to specify the credentials store in `$HOME/.docker/config.json`
to tell the docker engine to use it. The value of the config property should be
the suffix of the program to use (i.e. everything after `docker-credential-`).
For example, to use `docker-credential-osxkeychain`:

```json
{
	"credsStore": "osxkeychain"
}
```

If you are currently logged in, run `docker logout` to remove
the credentials from the file and run `docker login` again.

### Credential helper protocol

Credential helpers can be any program or script that follows a very simple protocol.
This protocol is heavily inspired by Git, but it differs in the information shared.

The helpers always use the first argument in the command to identify the action.
There are only three possible values for that argument: `store`, `get`, and `erase`.

The `store` command takes a JSON payload from the standard input. That payload carries
the server address, to identify the credential, the user name, and either a password
or an identity token.

```json
{
	"ServerURL": "https://index.docker.io/v1",
	"Username": "david",
	"Secret": "passw0rd1"
}
```

If the secret being stored is an identity token, the Username should be set to
`<token>`.

The `store` command can write error messages to `STDOUT` that the docker engine
will show if there was an issue.

The `get` command takes a string payload from the standard input. That payload carries
the server address that the docker engine needs credentials for. This is
an example of that payload: `https://index.docker.io/v1`.

The `get` command writes a JSON payload to `STDOUT`. Docker reads the user name
and password from this payload:

```json
{
	"Username": "david",
	"Secret": "passw0rd1"
}
```

The `erase` command takes a string payload from `STDIN`. That payload carries
the server address that the docker engine wants to remove credentials for. This is
an example of that payload: `https://index.docker.io/v1`.

The `erase` command can write error messages to `STDOUT` that the docker engine
will show if there was an issue.

### Credential helpers

Credential helpers are similar to the credential store above, but act as the
designated programs to handle credentials for *specific registries*. The default
credential store (`credsStore` or the config file itself) will not be used for
operations concerning credentials of the specified registries.

### Logging out

If you are currently logged in, run `docker logout` to remove
the credentials from the default store.

Credential helpers are specified in a similar way to `credsStore`, but
allow for multiple helpers to be configured at a time. Keys specify the
registry domain, and values specify the suffix of the program to use
(i.e. everything after `docker-credential-`).
For example:

```json
{
  "credHelpers": {
    "registry.example.com": "registryhelper",
    "awesomereg.example.org": "hip-star",
    "unicorn.example.io": "vcbait"
  }
}
```
