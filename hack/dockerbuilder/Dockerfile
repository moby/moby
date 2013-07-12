# DESCRIPTION     Build a container capable of producing official binary and
#                 PPA packages and uploading them to S3 and Launchpad
# VERSION         1.2
# DOCKER_VERSION  0.4
# AUTHOR          Solomon Hykes <solomon@dotcloud.com>
#                 Daniel Mizyrycki <daniel@dotcloud.net>
# BUILD_CMD       docker build -t dockerbuilder .
# RUN_CMD         docker run -e AWS_ID="$AWS_ID" -e AWS_KEY="$AWS_KEY" -e GPG_KEY="$GPG_KEY" -e PUBLISH_PPA="$PUBLISH_PPA" dockerbuilder
#
# ENV_VARIABLES   AWS_ID, AWS_KEY: S3 credentials for uploading Docker binary and tarball
#                 GPG_KEY: Signing key for docker package
#                 PUBLISH_PPA: 0 for staging release, 1 for production release
#
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>
# Workaround the upstart issue
run dpkg-divert --local --rename --add /sbin/initctl
run ln -s /bin/true /sbin/initctl
# Enable universe and gophers PPA
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q python-software-properties
run	add-apt-repository "deb http://archive.ubuntu.com/ubuntu $(lsb_release -sc) universe"
run	add-apt-repository -y ppa:dotcloud/docker-golang/ubuntu
run	apt-get update
# Packages required to checkout, build and upload docker
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q s3cmd curl
run	curl -s -o /go.tar.gz https://go.googlecode.com/files/go1.1.1.linux-amd64.tar.gz
run	tar -C /usr/local -xzf /go.tar.gz
run	echo "export PATH=/usr/local/go/bin:$PATH" > /.bashrc
run	echo "export PATH=/usr/local/go/bin:$PATH" > /.bash_profile
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q git build-essential
# Packages required to build an ubuntu package
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q golang-stable debhelper autotools-dev devscripts
# Copy dockerbuilder files into the container
add	.       /src
run	cp /src/dockerbuilder /usr/local/bin/ && chmod +x /usr/local/bin/dockerbuilder
cmd	["dockerbuilder"]
