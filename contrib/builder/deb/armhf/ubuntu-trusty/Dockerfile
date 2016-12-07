FROM armhf/ubuntu:trusty

RUN apt-get update && apt-get install -y apparmor bash-completion btrfs-tools build-essential curl ca-certificates debhelper dh-apparmor dh-systemd git libapparmor-dev libdevmapper-dev libltdl-dev libsqlite3-dev pkg-config libsystemd-journal-dev --no-install-recommends && rm -rf /var/lib/apt/lists/*

ENV GO_VERSION 1.6.4
RUN curl -fSL "https://storage.googleapis.com/golang/go${GO_VERSION}.linux-armv6l.tar.gz" | tar xzC /usr/local
ENV PATH $PATH:/usr/local/go/bin

ENV AUTO_GOPATH 1

ENV DOCKER_BUILDTAGS apparmor pkcs11 selinux
ENV RUNC_BUILDTAGS apparmor selinux
