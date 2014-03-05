title
:   Running an SSH service

description
:   Installing and running an sshd service

keywords
:   docker, example, package installation, networking

SSH Daemon Service
==================

The following Dockerfile sets up an sshd service in a container that you
can use to connect to and inspect other container's volumes, or to get
quick access to a test container.

Build the image using:

~~~~ {.sourceCode .bash}
$ sudo docker build -rm -t eg_sshd .
~~~~

Then run it. You can then use `docker port` to find out what host port
the container's port 22 is mapped to:

~~~~ {.sourceCode .bash}
$ sudo docker run -d -P -name test_sshd eg_sshd
$ sudo docker port test_sshd 22
0.0.0.0:49154
~~~~

And now you can ssh to port `49154` on the Docker daemon's host IP
address (`ip address` or `ifconfig` can tell you that):

~~~~ {.sourceCode .bash}
$ ssh root@192.168.1.2 -p 49154
# The password is ``screencast``.
$$
~~~~

Finally, clean up after your test by stopping and removing the
container, and then removing the image.

~~~~ {.sourceCode .bash}
$ sudo docker stop test_sshd
$ sudo docker rm test_sshd
$ sudo docker rmi eg_sshd
~~~~
