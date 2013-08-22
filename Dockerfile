# This file describes the standard way to build Docker, using docker
docker-version 0.4.2
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>
# Build dependencies
run	echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
run	apt-get update
run	apt-get install -y -q curl
run     apt-get install -y -q git
run     apt-get install -y -q mercurial
# Install Go
run	curl -s https://go.googlecode.com/files/go1.1.1.linux-amd64.tar.gz | tar -v -C /usr/local -xz
env	PATH	/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
env	GOPATH	/go:/vendor
env	CGO_ENABLED 0
run	cd /tmp && echo 'package main' > t.go && go test -a -i -v
# Run dependencies
run	apt-get install -y iptables
run	apt-get install -y lxc
run	apt-get install -y aufs-tools
# Upload docker source
add	.       /go/src/github.com/dotcloud/docker
# Upload vendored dependencies
add     vendor  /vendor
# Build the binary
run	cd /go/src/github.com/dotcloud/docker/docker && go install -ldflags "-X main.GITCOMMIT '??' -d -w"
env	PATH	/usr/local/go/bin:/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
cmd	["docker"]
