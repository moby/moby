FROM alpine:3.7
ENV PACKAGES="\
    musl \
    linux-headers \
    build-base \
    util-linux \
    bash \
    git \
    ca-certificates \
    python2 \
    python2-dev \
    py-setuptools \
    iproute2 \
    curl \
    strace \
    drill \
    ipvsadm \
    iperf \
    ethtool \
"

RUN echo \
    && apk add --no-cache $PACKAGES \
    && if [[ ! -e /usr/bin/python ]];        then ln -sf /usr/bin/python2.7 /usr/bin/python; fi \
    && if [[ ! -e /usr/bin/python-config ]]; then ln -sf /usr/bin/python2.7-config /usr/bin/python-config; fi \
    && if [[ ! -e /usr/bin/easy_install ]];  then ln -sf /usr/bin/easy_install-2.7 /usr/bin/easy_install; fi \
    && easy_install pip \
    && pip install --upgrade pip \
    && if [[ ! -e /usr/bin/pip ]]; then ln -sf /usr/bin/pip2.7 /usr/bin/pip; fi \
    && echo

ADD ssd.py /
RUN pip install git+git://github.com/docker/docker-py.git
ENTRYPOINT [ "python", "/ssd.py"]
