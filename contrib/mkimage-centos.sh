#!/bin/bash
# Create a CentOS base image for Docker
# From unclejack https://github.com/dotcloud/docker/issues/290
set -e

MIRROR_URL="http://centos.netnitco.net/6.4/os/x86_64/"
MIRROR_URL_UPDATES="http://centos.netnitco.net/6.4/updates/x86_64/"

yum install -y febootstrap xz

febootstrap -i bash -i coreutils -i tar -i bzip2 -i gzip -i vim-minimal -i wget -i patch -i diffutils -i iproute -i yum centos centos64  $MIRROR_URL -u $MIRROR_URL_UPDATES
touch centos64/etc/resolv.conf
touch centos64/sbin/init

tar --numeric-owner -Jcpf centos-64.tar.xz -C centos64 .
