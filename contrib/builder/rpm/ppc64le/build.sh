#!/usr/bin/env bash
set -e

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"

wget http://download.opensuse.org/repositories/Virtualization:/containers:/images:/openSUSE-Tumbleweed/images/openSUSE-Tumbleweed-docker-guest-docker.ppc64le.tar.xz && unxz openSUSE-Tumbleweed-docker-guest-docker.ppc64le.tar.xz && docker load -i openSUSE-Tumbleweed-docker-guest-docker.ppc64le.tar 

set -x
./generate.sh
for d in */; do
	docker build -t "dockercore/builder-rpm:$(basename "$d")" "$d"
done
