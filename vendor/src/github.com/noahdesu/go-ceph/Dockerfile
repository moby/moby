FROM golang:1.3-wheezy
MAINTAINER Abhishek Lekshmanan "abhishek.lekshmanan@gmail.com"

ENV CEPH_VERSION giant

RUN echo deb http://ceph.com/debian-$CEPH_VERSION/ wheezy main | tee /etc/apt/sources.list.d/ceph-$CEPH_VERSION.list

# Running wget with no certificate checks, alternatively ssl-cert package should be installed
RUN wget --no-check-certificate -q -O- 'https://ceph.com/git/?p=ceph.git;a=blob_plain;f=keys/release.asc' | apt-key add - &&\
    apt-get update && \
    apt-get install -y ceph \
    librados-dev librbd-dev libcephfs-dev

VOLUME /go/src/github.com/noahdesu/go-ceph

COPY ./ci/entrypoint.sh /tmp/entrypoint.sh

ENTRYPOINT ["/tmp/entrypoint.sh", "/tmp/micro-ceph"]

