# This will build a container capable of producing an official binary build of docker and
# uploading it to S3
from	ubuntu:12.04
maintainer	Solomon Hykes <solomon@dotcloud.com>
# Workaround the upstart issue
run dpkg-divert --local --rename --add /sbin/initctl
run ln -s /bin/true /sbin/initctl
# Enable universe and gophers PPA
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q python-software-properties
run	add-apt-repository "deb http://archive.ubuntu.com/ubuntu $(lsb_release -sc) universe"
run	add-apt-repository -y ppa:gophers/go/ubuntu
run	apt-get update
# Packages required to checkout, build and upload docker
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q s3cmd
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q curl
run	curl -s -o /go.tar.gz https://go.googlecode.com/files/go1.1.1.linux-amd64.tar.gz
run	tar -C /usr/local -xzf /go.tar.gz
run	echo "export PATH=/usr/local/go/bin:$PATH" > /.bashrc
run	echo "export PATH=/usr/local/go/bin:$PATH" > /.bash_profile
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q git
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q build-essential
# Packages required to build an ubuntu package
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q golang-stable
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q debhelper
run	DEBIAN_FRONTEND=noninteractive apt-get install -y -q autotools-dev
run	apt-get install -y -q devscripts
# Copy dockerbuilder files into the container
add	.       /src
run	cp /src/dockerbuilder /usr/local/bin/ && chmod +x /usr/local/bin/dockerbuilder
run	cp /src/s3cfg /.s3cfg
cmd	["dockerbuilder"]
