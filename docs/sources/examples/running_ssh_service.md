page_title: Dockerizing an SSH service
page_description: Installing and running an SSHd service on Docker
page_keywords: docker, example, package installation, networking

# Dockerizing an SSH Daemon Service

The following `Dockerfile` sets up an SSHd service in a container that you
can use to connect to and inspect other container's volumes, or to get
quick access to a test container.

    # sshd
    #
    # VERSION               0.0.1

    FROM     ubuntu:12.04
    MAINTAINER Thatcher R. Peskens "thatcher@dotcloud.com"

    RUN apt-get update && apt-get install -y openssh-server
    RUN mkdir /var/run/sshd
    RUN echo 'root:screencast' |chpasswd

    EXPOSE 22
    CMD    ["/usr/sbin/sshd", "-D"]

Build the image using:

    $ sudo docker build -t eg_sshd .

Then run it. You can then use `docker port` to find out what host port
the container's port 22 is mapped to:

    $ sudo docker run -d -P --name test_sshd eg_sshd
    $ sudo docker port test_sshd 22
    0.0.0.0:49154

And now you can ssh to port `49154` on the Docker daemon's host IP
address (`ip address` or `ifconfig` can tell you that):

    $ ssh root@192.168.1.2 -p 49154
    # The password is ``screencast``.
    $$

Finally, clean up after your test by stopping and removing the
container, and then removing the image.

    $ sudo docker stop test_sshd
    $ sudo docker rm test_sshd
    $ sudo docker rmi eg_sshd

