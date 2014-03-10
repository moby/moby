# sshd
#
# VERSION               0.0.1

FROM     ubuntu
MAINTAINER Thatcher R. Peskens "thatcher@dotcloud.com"

# make sure the package repository is up to date
RUN echo "deb http://archive.ubuntu.com/ubuntu precise main universe" > /etc/apt/sources.list
RUN apt-get update

RUN apt-get install -y openssh-server
RUN mkdir /var/run/sshd 
RUN echo 'root:screencast' |chpasswd

EXPOSE 22
CMD    /usr/sbin/sshd -D
