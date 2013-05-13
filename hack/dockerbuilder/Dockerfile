# This will build a container capable of producing an official binary build of docker and
# uploading it to S3
maintainer	Solomon Hykes <solomon@dotcloud.com>
from	ubuntu:12.10
run	apt-get update
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q s3cmd
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q curl
# Packages required to checkout and build docker
run	curl -s -o /go.tar.gz https://go.googlecode.com/files/go1.1.linux-amd64.tar.gz
run	tar -C /usr/local -xzf /go.tar.gz
run	echo "export PATH=$PATH:/usr/local/go/bin" > /.bashrc
run	echo "export PATH=$PATH:/usr/local/go/bin" > /.bash_profile
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q git
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q build-essential
# Packages required to build an ubuntu package
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q debhelper
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q autotools-dev
copy	fake_initctl	/usr/local/bin/initctl
run	apt-get install -y -q devscripts
add	.	/src
run	cp /src/dockerbuilder /usr/local/bin/ && chmod +x /usr/local/bin/dockerbuilder
run	cp /src/s3cfg /.s3cfg
cmd	["dockerbuilder"]
