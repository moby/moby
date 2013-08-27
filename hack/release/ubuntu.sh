# Set variables for the Ubuntu packaging

PACKAGE_URL="http://www.docker.io/"
PACKAGE_MAINTAINER="docker@dotcloud.com"
PACKAGE_DESCRIPTION="lxc-docker is a Linux container runtime
Docker complements LXC with a high-level API which operates at the process
level. It runs unix processes with strong guarantees of isolation and
repeatability across servers.
Docker is a great building block for automating distributed systems:
large-scale web deployments, database clusters, continuous deployment systems,
private PaaS, service-oriented architectures, etc."
PACKAGE_ARCHITECTURE="$(dpkg-architecture -qDEB_HOST_ARCH)"
UPSTART_SCRIPT='description     "Docker daemon"

start on filesystem or runlevel [2345]
stop on runlevel [!2345]

respawn

script
    /usr/bin/docker -d
end script
'
