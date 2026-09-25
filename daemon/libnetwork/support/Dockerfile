# syntax=docker/dockerfile:1

FROM docker:29-cli

RUN apk add --no-cache \
    util-linux \
    bridge-utils \
    iptables \
    iputils \
    iproute2 \
    ipvsadm \
    conntrack-tools \
    jq \
    bash

WORKDIR /bin
COPY *.sh /bin/

CMD ["/bin/run.sh"]
