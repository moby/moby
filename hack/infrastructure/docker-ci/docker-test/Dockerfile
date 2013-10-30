# VERSION:        0.3
# DOCKER-VERSION  0.6.3
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    Testing docker PRs and commits on top of master using
# REFERENCES:     This code reuses the excellent implementation of
#                 Docker in Docker made by Jerome Petazzoni.
#                 https://github.com/jpetazzo/dind
# COMMENTS:
#   This Dockerfile adapts /Dockerfile to enable docker PRs and commits testing
#   Optional arguments:
#       [commit]  (default: 'HEAD')
#       [repo]    (default: 'http://github.com/dotcloud/docker')
#       [branch]  (default: 'master')
# TO_BUILD:       docker build -t test_docker .
# TO_RUN:         docker run -privileged test_docker hack/dind test_docker.sh [commit] [repo] [branch]

from docker
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

#### FIXME. Temporarily install docker and dind with proper apparmor handling
run wget -q -O /go/src/github.com/dotcloud/docker/hack/dind http://raw.github.com/jpetazzo/docker/escape-apparmor-confinement/hack/dind
run chmod +x /go/src/github.com/dotcloud/docker/hack/dind

# Setup go to the PATH. Extracted from /Dockerfile
env PATH /usr/local/go/bin:$PATH

# Add test_docker.sh
add test_docker.sh /usr/bin/test_docker.sh
run chmod +x /usr/bin/test_docker.sh
