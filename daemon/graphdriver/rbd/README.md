Ceph Graph Driver Documentation
===============================

# Known issues

 - At the moment it is *not* possible to statically compile docker with
   the ceph graph driver enabled because some static libraries are missing
   (Ubuntu 14.04). Please use: ./hack/make.sh dynbinary

# How to compile

With Ubuntu 14.04:

```bash
# cd /root/go/src/github.com/docker
# git clone https://github.com/docker/docker.git && cd docker
# export GOPATH=/root/go/src/github.com/docker/docker/vendor/:/root/go

# hack/make.sh dynbinary
# WARNING! I don't seem to be running in the Docker container.
# The result of this command might be an incorrect build, and will not be
#   officially supported.
#
# Try this instead: make all
#

bundles/1.8.0-dev already exists. Removing.

---> Making bundle: dynbinary (in bundles/1.8.0-dev/dynbinary)
Created binary: bundles/1.8.0-dev/dynbinary/dockerinit-1.8.0-dev
Building: bundles/1.8.0-dev/dynbinary/docker-1.8.0-dev
Created binary: bundles/1.8.0-dev/dynbinary/docker-1.8.0-dev
```
# How to use

## install ceph cluster
TODO:

## run docker daemon

```bash
# cp bundles/1.8.0-dev/dynbinary/docker-1.8.0-dev /usr/bin/docker
# cp bundles/1.8.0-dev/dynbinary/dockerinit-1.8.0-dev /var/lib/docker/
# docker -d -D -s rbd
...
```
## pull images

```bash
# docker pull centos:latest
Pulling repository centos
7322fbe74aa5: Download complete 
f1b10cd84249: Download complete 
c852f6d61e65: Download complete 
Status: Downloaded newer image for centos:latest
```

## list rbd image

```bash
# rbd list
docker_image_7322fbe74aa5632b33a400959867c8ac4290e9c5112877a7754be70cfe5d66e9
docker_image_base_image
docker_image_c852f6d61e65cddf1e8af1f6cd7db78543bfb83cdcd36845541cf6d9dfef20a0
docker_image_f1b10cd842498c23d206ee0cbeaa9de8d2ae09ff3c7af2723a9e337a6965d639
```
## run container

```bash
# docker run -it --rm centos:latest /bin/bash
[root@290238155b54 /]#
```

```bash
# rbd list
docker_image_290238155b547852916b732e38bc4494375e1ed2837272e2940dfccc62691f6c
docker_image_290238155b547852916b732e38bc4494375e1ed2837272e2940dfccc62691f6c-init
docker_image_7322fbe74aa5632b33a400959867c8ac4290e9c5112877a7754be70cfe5d66e9
docker_image_base_image
docker_image_c852f6d61e65cddf1e8af1f6cd7db78543bfb83cdcd36845541cf6d9dfef20a0
docker_image_f1b10cd842498c23d206ee0cbeaa9de8d2ae09ff3c7af2723a9e337a6965d639
```
