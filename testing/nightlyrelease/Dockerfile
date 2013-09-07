# VERSION:        1.0
# DOCKER-VERSION  0.6.1
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Build docker nightly release using Docker in Docker.
# REFERENCES:     This code reuses the excellent implementation of docker in docker
#                 made by Jerome Petazzoni.  https://github.com/jpetazzo/dind
# COMMENTS:
#   release_credentials.json is a base64 json encoded file containing:
#       { "AWS_ACCESS_KEY": "Test_docker_AWS_S3_bucket_id",
#         "AWS_SECRET_KEY='Test_docker_AWS_S3_bucket_key'
#         "GPG_PASSPHRASE='Test_docker_GPG_passphrase_signature'
#         "INDEX_AUTH='Encripted_index_authentication' }
#   When releasing:  Docker in Docker requires cgroups mounted the same way in
#   the host and containers:
#       stop docker
#       umount /sys/fs/cgroup/*; umount /sys/fs/cgroup; mount -t tmpfs none /sys/fs/cgroup
#       cd /sys/fs/cgroup; for C in $(awk '{print $1}' < /proc/cgroups | grep -v subsys | grep -v memory) ; do mkdir $C ; mount -t cgroup none -o $C $C ; done
#       docker -d &
# TO_BUILD:       docker build -t dockerbuilder .
# TO_RELEASE:     docker run -i -t -privileged -lxc-conf="lxc.aa_profile = unconfined" -e AWS_S3_BUCKET="test.docker.io" dockerbuilder

from ubuntu:12.04
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

# Add docker dependencies
run echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
run apt-get update; apt-get install -y -q iptables ca-certificates bzip2 python lxc curl git mercurial
run curl -s https://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | tar -v -C /usr/local -xz
run ln -s /usr/local/go/bin/go /usr/bin

# Add production docker binary
run curl http://get.docker.io/builds/Linux/x86_64/docker-latest >/usr/bin/docker; chmod +x /usr/bin/docker

# Add proto docker builder
add ./dockerbuild /usr/bin/dockerbuild
run chmod +x /usr/bin/dockerbuild

# Add release credentials
add ./release_credentials.json /root/release_credentials.json

# Make /tmp and /var/lib/docker inside the container addressable by other containers.
# This is done to ensure /tmp and /var/lib/docker has AUFS support needed by the inner docker server
volume /tmp
volume /var/lib/docker

# Launch build process in a container
cmd dockerbuild
