FROM docker:18-dind

RUN set -ex \
    && echo "http://nl.alpinelinux.org/alpine/edge/main" >> /etc/apk/repositories \
    && echo "http://nl.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories
RUN apk add --no-cache \
    util-linux \
    bridge-utils \
    iptables \
    iputils \
    iproute2 \
    ipvsadm \
    conntrack-tools \
    bash

WORKDIR /bin
COPY *.sh /bin/

CMD /bin/run.sh
