# This file describes the standard way to build Docker, using docker
docker-version 0.4.2
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>
# Build dependencies
run	apt-get install -y -q curl
run	apt-get install -y -q git
# Install Go
run	curl -s https://go.googlecode.com/files/go1.1.1.linux-amd64.tar.gz | tar -v -C /usr/local -xz
env	PATH	/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
env	GOPATH	/go
env	CGO_ENABLED 0
run	cd /tmp && echo 'package main' > t.go && go test -a -i -v
# Download dependencies
run	PKG=github.com/kr/pty REV=27435c699;		 git clone http://$PKG /go/src/$PKG && cd /go/src/$PKG && git checkout -f $REV
run	PKG=github.com/gorilla/context/ REV=708054d61e5; git clone http://$PKG /go/src/$PKG && cd /go/src/$PKG && git checkout -f $REV
run	PKG=github.com/gorilla/mux/ REV=9b36453141c;	 git clone http://$PKG /go/src/$PKG && cd /go/src/$PKG && git checkout -f $REV
# Run dependencies
run	apt-get install -y iptables
# lxc requires updating ubuntu sources
run	echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
run	apt-get update
run	apt-get install -y lxc
run	apt-get install -y aufs-tools
# Docker requires code.google.com/p/go.net/websocket
run	apt-get install -y -q mercurial
run	PKG=code.google.com/p/go.net REV=78ad7f42aa2e;	 hg clone https://$PKG /go/src/$PKG && cd /go/src/$PKG && hg checkout -r $REV
# Upload docker source
add	.       /go/src/github.com/dotcloud/docker
# Build the binary
run	cd /go/src/github.com/dotcloud/docker/docker && go install -ldflags "-X main.GITCOMMIT '??' -d -w"
env	PATH	/usr/local/go/bin:/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
cmd	["docker"]
