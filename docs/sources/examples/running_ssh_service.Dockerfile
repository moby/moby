# sshd
#
# VERSION               0.0.1

FROM    ubuntu:12.04
MAINTAINER Thatcher R. Peskens "thatcher@dotcloud.com"

RUN apt-get update && apt-get install -y openssh-server
RUN mkdir /var/run/sshd
RUN echo 'root:screencast' |chpasswd

EXPOSE 22
CMD    ["/usr/sbin/sshd", "-D"]
