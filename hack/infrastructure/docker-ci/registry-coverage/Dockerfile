# VERSION:        0.1
# DOCKER-VERSION  0.6.4
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Docker registry coverage
# COMMENTS:       Add registry coverage into the docker-ci image
# TO_BUILD:       docker build -t registry_coverage .
# TO_RUN:         docker run registry_coverage

from docker-ci
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

# Add registry_coverager.sh and dependencies
run pip install coverage flask pyyaml requests simplejson python-glanceclient \
    blinker redis gevent
add registry_coverage.sh /usr/bin/registry_coverage.sh
run chmod +x /usr/bin/registry_coverage.sh

cmd "/usr/bin/registry_coverage.sh"
