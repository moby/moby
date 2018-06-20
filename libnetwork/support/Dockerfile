FROM docker:18-dind

RUN set -ex \
    && echo "http://nl.alpinelinux.org/alpine/edge/main" >> /etc/apk/repositories \
    && echo "http://nl.alpinelinux.org/alpine/edge/testing" >> /etc/apk/repositories \
    && apk update \
    && apk add --no-cache \
    tcpdump \
    bridge-utils \
    netcat-openbsd \
    util-linux \
    iptables \
    iputils \
    iproute2 \
    iftop \
    drill \
    apache2-utils \
    strace \
    curl \
    ethtool \
    ipvsadm \
    ngrep \
    iperf \
    nmap \
    nmap-nping \
    conntrack-tools \
    socat \
    busybox-extras \
    tcptraceroute \
    mtr \
    fping \
    liboping \
    iptraf-ng \
    dhcping \
    nmap-nping \
    net-snmp-tools \
    python2 \
    py2-virtualenv \
    py-crypto \
    scapy \
    vim \
    bird \
    bash \
    bind-tools

WORKDIR /bin
COPY support.sh .

CMD /bin/support.sh
