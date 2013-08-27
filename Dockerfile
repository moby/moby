# This file describes the standard way to build Docker, using docker
docker-version 0.4.2
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>
# Build dependencies
run	echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
run	apt-get update
run	apt-get install -y -q curl
run	apt-get install -y -q git
run	apt-get install -y -q mercurial
# Install Go
run	curl -s https://go.googlecode.com/files/go1.1.2.linux-amd64.tar.gz | tar -v -C /usr/local -xz
env	PATH	/usr/local/go/bin:/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin
env	GOPATH	/go:/vendor
run	cd /tmp && echo 'package main' > t.go && go test -a -i -v
# Ubuntu stuff
run	apt-get install -y -q ruby1.9.3 rubygems libffi-dev
run	gem install fpm
run	apt-get install -y -q reprepro dpkg-sig
# Install s3cmd 1.0.1 (earlier versions don't support env variables in the config)
run	apt-get install -y -q python-pip
run	pip install s3cmd
run	pip install python-magic
run	/bin/echo -e '[default]\naccess_key=$AWS_ACCESS_KEY\nsecret_key=$AWS_SECRET_KEY\n' > /.s3cfg
# Upload docker source
add	.       /go/src/github.com/dotcloud/docker
run	ln -s	/go/src/github.com/dotcloud/docker /src
# Setup vendor
run     cd /go/src/github.com/dotcloud/docker && ./vendor.sh
run    ln -s   /go/src/github.com/dotcloud/docker/vendor /vendor
# Build the binary
run	cd /go/src/github.com/dotcloud/docker && hack/release/make.sh
cmd	cd /go/src/github.com/dotcloud/docker && hack/release/release.sh
