# DOCKER-VERSION: 0.7.6
# AUTHOR:         Daniel Mizyrycki <daniel@dotcloud.com>
# DESCRIPTION:    docker-ci continuous integration service
# TO_BUILD:       docker build -t docker-ci/docker-ci .
# TO_RUN:         docker run --rm -i -t -p 8000:80 -p 2222:22 -v /run:/var/socket \
#                     -v /data/docker-ci:/data/docker-ci docker-ci/docker-ci

from ubuntu:12.04
maintainer Daniel Mizyrycki <daniel@dotcloud.com>

ENV DEBIAN_FRONTEND noninteractive
RUN echo 'deb http://archive.ubuntu.com/ubuntu precise main universe' > \
    /etc/apt/sources.list; apt-get update
RUN apt-get install -y --no-install-recommends  python2.7 python-dev \
    libevent-dev git supervisor ssh rsync less vim sudo gcc wget nginx
RUN cd /tmp; wget http://python-distribute.org/distribute_setup.py
RUN cd /tmp; python distribute_setup.py; easy_install pip; rm distribute_setup.py

RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 36A1D7869245C8950F966E92D8576A8BA88D21E9
RUN echo 'deb http://get.docker.io/ubuntu docker main' > \
      /etc/apt/sources.list.d/docker.list; apt-get update
RUN apt-get install -y lxc-docker-0.8.0
RUN pip install SQLAlchemy==0.7.10 buildbot buildbot-slave pyopenssl boto
RUN ln -s /var/socket/docker.sock /run/docker.sock

ADD . /docker-ci
RUN /docker-ci/setup.sh

ENTRYPOINT ["supervisord", "-n"]
