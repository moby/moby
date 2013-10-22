:title: History Command
:description: Show the history of an image
:keywords: history, docker, container, documentation

===========================================
``history`` -- Show the history of an image
===========================================

::

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image

Examples
--------

To see how the docker:latest image is built:

.. code-block:: bash

	$ docker history docker
	ID                  CREATED             CREATED BY
	docker:latest       19 hours ago        /bin/sh -c #(nop) ADD . in /go/src/github.com/dotcloud/docker
	cf5f2467662d        2 weeks ago         /bin/sh -c #(nop) ENTRYPOINT ["hack/dind"]
	3538fbe372bf        2 weeks ago         /bin/sh -c #(nop) WORKDIR /go/src/github.com/dotcloud/docker
	7450f65072e5        2 weeks ago         /bin/sh -c #(nop) VOLUME /var/lib/docker
	b79d62b97328        2 weeks ago         /bin/sh -c apt-get install -y -q lxc
	36714852a550        2 weeks ago         /bin/sh -c apt-get install -y -q iptables
	8c4c706df1d6        2 weeks ago         /bin/sh -c /bin/echo -e '[default]\naccess_key=$AWS_ACCESS_KEY\nsecret_key=$AWS_SECRET_KEY\n' > /.s3cfg
	b89989433c48        2 weeks ago         /bin/sh -c pip install python-magic
	a23e640d85b5        2 weeks ago         /bin/sh -c pip install s3cmd
	41f54fec7e79        2 weeks ago         /bin/sh -c apt-get install -y -q python-pip
	d9bc04add907        2 weeks ago         /bin/sh -c apt-get install -y -q reprepro dpkg-sig
	e74f4760fa70        2 weeks ago         /bin/sh -c gem install --no-rdoc --no-ri fpm
	1e43224726eb        2 weeks ago         /bin/sh -c apt-get install -y -q ruby1.9.3 rubygems libffi-dev
	460953ae9d7f        2 weeks ago         /bin/sh -c #(nop) ENV GOPATH=/go:/go/src/github.com/dotcloud/docker/vendor
	8b63eb1d666b        2 weeks ago         /bin/sh -c #(nop) ENV PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/goroot/bin
	3087f3bcedf2        2 weeks ago         /bin/sh -c #(nop) ENV GOROOT=/goroot
	635840d198e5        2 weeks ago         /bin/sh -c cd /goroot/src && ./make.bash
	439f4a0592ba        2 weeks ago         /bin/sh -c curl -s https://go.googlecode.com/files/go1.1.2.src.tar.gz | tar -v -C / -xz && mv /go /goroot
	13967ed36e93        2 weeks ago         /bin/sh -c #(nop) ENV CGO_ENABLED=0
	bf7424458437        2 weeks ago         /bin/sh -c apt-get install -y -q build-essential
	a89ec997c3bf        2 weeks ago         /bin/sh -c apt-get install -y -q mercurial
	b9f165c6e749        2 weeks ago         /bin/sh -c apt-get install -y -q git
	17a64374afa7        2 weeks ago         /bin/sh -c apt-get install -y -q curl
	d5e85dc5b1d8        2 weeks ago         /bin/sh -c apt-get update
	13e642467c11        2 weeks ago         /bin/sh -c echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > /etc/apt/sources.list
	ae6dde92a94e        2 weeks ago         /bin/sh -c #(nop) MAINTAINER Solomon Hykes <solomon@dotcloud.com>
	ubuntu:12.04        6 months ago 

