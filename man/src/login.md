Log in to a Docker Registry located on the specified
`SERVER`.  You can specify a URL or a `hostname` for the `SERVER` value. If you
do not specify a `SERVER`, the command uses Docker's public registry located at
`https://registry-1.docker.io/` by default.  To get a username/password for Docker's public registry, create an account on Docker Hub.

`docker login` requires user to use `sudo` or be `root`, except when:

1.  connecting to  a remote daemon, such as a `docker-machine` provisioned `docker engine`.
2.  user is added to the `docker` group.  This will impact the security of your system; the `docker` group is `root` equivalent.  See [Docker Daemon Attack Surface](https://docs.docker.com/engine/security/security/#/docker-daemon-attack-surface) for details.

You can log into any public or private repository for which you have
credentials.  When you log in, the command stores encoded credentials in
`$HOME/.docker/config.json` on Linux or `%USERPROFILE%/.docker/config.json` on Windows.

# EXAMPLES

## Login to a registry on your localhost

    # docker login localhost:8080

# See also
**docker-logout(1)** to log out from a Docker registry.
