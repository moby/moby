# VERSION:        0.1
# DOCKER-VERSION  0.6.4
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Docker registry coverage
# COMMENTS:       Add registry coverage into the docker-ci image
# TO_BUILD:       docker build -t registry-coverage .
# TO_RUN:         docker run registry-coverage

from docker-ci
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

# Add registry-coverage.sh and dependencies
run pip install coverage flask pyyaml requests simplejson python-glanceclient \
    blinker redis boto gevent rsa mock
add registry-coverage.sh /usr/bin/registry-coverage.sh
run chmod +x /usr/bin/registry-coverage.sh

cmd ["/usr/bin/registry-coverage.sh"]
