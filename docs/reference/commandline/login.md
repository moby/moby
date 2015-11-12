<!--[metadata]>
+++
title = "login"
description = "The login command description and usage"
keywords = ["registry, login, image"]
[menu.main]
parent = "smn_cli"
+++
<![end-metadata]-->

# login

    Usage: docker login [OPTIONS] [SERVER]

    Register or log in to a Docker registry server, if no server is
	specified "https://index.docker.io/v1/" is the default.

      -e, --email=""       Email
      --help=false         Print usage
      -p, --password=""    Password
      -u, --username=""    Username

If you want to login to a self-hosted registry you can specify this by
adding the server name.

    example:
    $ docker login localhost:8080


`docker login` requires user to use `sudo` or be `root`, except when: 

1.  connecting to a remote daemon, such as a `docker-machine` provisioned `docker engine`.
2.  user is added to the `docker` group.  This will impact the security of your system; the `docker` group is `root` equivalent.  See [Docker Daemon Attack Surface](https://docs.docker.com/articles/security/#docker-daemon-attack-surface) for details. 

You can log into any public or private repository for which you have
credentials.  When you log in, the command stores encoded credentials in
`$HOME/.docker/config.json` on Linux or `%USERPROFILE%/.docker/config.json` on Windows.

> **Note**:  When running `sudo docker login` credentials are saved in `/root/.docker/config.json`.
>
