page_title: Installation on Windows
page_description: Please note this project is currently under heavy development. It should not be used in production.
page_keywords: Docker, Docker documentation, Windows, requirements, virtualbox, boot2docker

# Windows

Docker can run on Windows using a virtualization platform like
VirtualBox. A Linux distribution is run inside a virtual machine and
that’s where Docker will run.

## Installation

Note

Docker is still under heavy development! We don’t recommend using it in
production yet, but we’re getting closer with each release. Please see
our blog post, ["Getting to Docker
1.0"](http://blog.docker.io/2013/08/getting-to-docker-1-0/)

1.  Install virtualbox from
    [https://www.virtualbox.org](https://www.virtualbox.org) - or follow
    this
    [tutorial](http://www.slideshare.net/julienbarbier42/install-virtualbox-on-windows-7).
2.  Download the latest boot2docker.iso from
    [https://github.com/boot2docker/boot2docker/releases](https://github.com/boot2docker/boot2docker/releases).
3.  Start VirtualBox.
4.  Create a new Virtual machine with the following settings:

> -   Name: boot2docker
> -   Type: Linux
> -   Version: Linux 2.6 (64 bit)
> -   Memory size: 1024 MB
> -   Hard drive: Do not add a virtual hard drive

5.  Open the settings of the virtual machine:

    5.1. go to Storage

    5.2. click the empty slot below Controller: IDE

    5.3. click the disc icon on the right of IDE Secondary Master

    5.4. click Choose a virtual CD/DVD disk file

6.  Browse to the path where you’ve saved the boot2docker.iso, select
    the boot2docker.iso and click open.

7.  Click OK on the Settings dialog to save the changes and close the
    window.

8.  Start the virtual machine by clicking the green start button.

9.  The boot2docker virtual machine should boot now.

## Running Docker

boot2docker will log you in automatically so you can start using Docker
right away.

Let’s try the “hello world” example. Run

    docker run busybox echo hello world

This will download the small busybox image and print hello world.

## Observations

### Persistent storage

The virtual machine created above lacks any persistent data storage. All
images and containers will be lost when shutting down or rebooting the
VM.
