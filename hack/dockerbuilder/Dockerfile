# This will build a container capable of producing an official binary build of docker and
# uploading it to S3
from	ubuntu:12.10
run	apt-get update
run	RUNLEVEL=1 DEBIAN_FRONTEND=noninteractive apt-get install -y -q s3cmd
run	RUNLEVEL=1 DEBIAN_FRONTEND=noninteractive apt-get install -y -q golang
run	RUNLEVEL=1 DEBIAN_FRONTEND=noninteractive apt-get install -y -q git
run	RUNLEVEL=1 DEBIAN_FRONTEND=noninteractive apt-get install -y -q build-essential
copy	dockerbuilder	/usr/local/bin/dockerbuilder
copy	s3cfg	/.s3cfg
# run $img dockerbuilder $REVISION_OR_TAG $S3_ID $S3_KEY
