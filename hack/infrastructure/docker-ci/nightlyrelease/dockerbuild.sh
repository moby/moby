#!/bin/bash

# Variables AWS_ACCESS_KEY, AWS_SECRET_KEY and PG_PASSPHRASE are decoded
# from /root/release_credentials.json
# Variable AWS_S3_BUCKET is passed to the environment from docker run -e

# Turn debug off to load credentials from the environment
set +x
eval $(cat /root/release_credentials.json  | python -c '
import sys,json,base64;
d=json.loads(base64.b64decode(sys.stdin.read()));
exec("""for k in d: print "export {0}=\\"{1}\\"".format(k,d[k])""")')

# Fetch docker master branch
set -x
cd /
rm -rf /go
git clone -q -b master http://github.com/dotcloud/docker /go/src/github.com/dotcloud/docker
cd /go/src/github.com/dotcloud/docker

# Launch docker daemon using dind inside the container
/usr/bin/docker version
/usr/bin/docker -d &
sleep 5

# Build Docker release container
docker build -t docker .

# Test docker and if everything works well, release
echo docker run -i -t -privileged -e AWS_S3_BUCKET=$AWS_S3_BUCKET -e AWS_ACCESS_KEY=XXXXX -e AWS_SECRET_KEY=XXXXX -e GPG_PASSPHRASE=XXXXX docker hack/release.sh
set +x
docker run -privileged -i -t -e AWS_S3_BUCKET=$AWS_S3_BUCKET -e AWS_ACCESS_KEY=$AWS_ACCESS_KEY -e AWS_SECRET_KEY=$AWS_SECRET_KEY -e GPG_PASSPHRASE=$GPG_PASSPHRASE docker hack/release.sh
exit_status=$?

# Display load if test fails
set -x
if [ $exit_status -ne 0 ] ; then
    uptime; echo; free
    exit 1
fi
