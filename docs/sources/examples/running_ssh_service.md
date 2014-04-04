page_title: Running an SSH service
page_description: Installing and running an sshd service
page_keywords: docker, example, package installation, networking

# SSH Daemon Service

> **Note:** 
> - This example assumes you have Docker running in daemon mode. For
>   more information please see [*Check your Docker
>   install*](../hello_world/#running-examples).
> - **If you don’t like sudo** then see [*Giving non-root
>   access*](../../installation/binaries/#dockergroup)

The following Dockerfile sets up an sshd service in a container that you
can use to connect to and inspect other container’s volumes, or to get
quick access to a test container.

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

Build the image using:

    $ sudo docker build -rm -t eg_sshd .

Then run it. You can then use `docker port` to find
out what host port the container’s port 22 is mapped to:

    $ sudo docker run -d -P -name test_sshd eg_sshd
    $ sudo docker port test_sshd 22
    0.0.0.0:49154

And now you can ssh to port `49154` on the Docker
daemon’s host IP address (`ip address` or
`ifconfig` can tell you that):

    $ ssh root@192.168.1.2 -p 49154
    # The password is ``screencast``.
    $$

Finally, clean up after your test by stopping and removing the
container, and then removing the image.

    $ sudo docker stop test_sshd
    $ sudo docker rm test_sshd
    $ sudo docker rmi eg_sshd
