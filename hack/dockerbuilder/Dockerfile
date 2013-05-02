# This will build a container capable of producing an official binary build of docker and
# uploading it to S3
maintainer	Solomon Hykes <solomon@dotcloud.com>
from	ubuntu:12.10
run	apt-get update
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q s3cmd
# Packages required to checkout and build docker
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q golang
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q git
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q build-essential
# Packages required to build an ubuntu package
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q debhelper
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q autotools-dev
add	.	/src
run	cp /src/dockerbuilder /usr/local/bin/
run	cp /src/fake_initctl /usr/local/bin/initctl
run	cp /src/s3cfg /.s3cfg
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q devscripts
cmd	dockerbuilder
