page_title: Configuring Docker
page_description: Configuring the Docker daemon on various distributions
page_keywords: docker, daemon, configuration

# Configuring Docker on various distributions

After successfully installing the Docker daemon on a distribution, it runs with it's default
config. Usually it is required to change the default config to meet one's personal requirements.
 
Docker can be configured by passing the config flags to the daemon directly if the daemon
is started directly. Usually that is not the case. A process manager (like SysVinit, Upstart,
systemd, etc) is responsible for starting and running the daemon.

Some common config options are

* `-D` : Enable debug mode

* `-H` : Daemon socket(s) to connect to   

* `--tls` : Enable or disable TLS authentication

The complete list of flags can found at [Docker Command Line Reference](/reference/commandline/cli/)

## Ubuntu

After successfully [installing Docker for Ubuntu](/installation/ubuntulinux/), you can check the
running status using (if running Upstart)

    $ sudo status docker
    docker start/running, process 989

You can start/stop/restart `docker` using

    $ sudo start docker

    $ sudo stop docker

    $ sudo restart docker


### Configuring Docker

Docker options can be configured by editing the file `/etc/default/docker`. If this file does not 
exist, it needs to be createdThis file contains a variable named `DOCKER_OPTS`. All the 
config options need to be placed in this variable. For example

    DOCKER_OPTS=" --dns 8.8.8.8 -D --tls=false -H tcp://0.0.0.0:2375 "

The above daemon options : 

1. Set dns server for all containers

2. Enable Debug mode 

3. Set tls to false

4. Make the daemon listen for connections on `tcp://0.0.0.0:2375`

After saving the file, restart docker using `sudo restart docker`. Verify that the daemon is
running with the options specified by running `ps aux | grep docker | grep -v grep`
