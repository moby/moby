# TO_BUILD:      docker build --no-cache -t docker-ci/testbuilder .
# TO_RUN:        docker run --rm -u sysadmin \
#                -v /run:/var/socket docker-ci/testbuilder docker-registry
#

FROM docker-ci/docker-ci
ENV HOME /home/sysadmin

RUN mkdir /testbuilder
ADD . /testbuilder

ENTRYPOINT ["/testbuilder/testbuilder.sh"]
