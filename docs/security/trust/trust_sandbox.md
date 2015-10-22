<!--[metadata]>
+++
title = "Play in a content trust sandbox"
description = "Play in a trust sandbox"
keywords = ["trust, security, root,  keys, repository, sandbox"]
[menu.main]
parent= "smn_content_trust"
+++
<![end-metadata]-->

# Play in a content trust sandbox

This page explains how to set up and use a sandbox for experimenting with trust.
The sandbox allows you to configure and try trust operations locally without
impacting your production images.

Before working through this sandbox, you should have read through the [trust
overview](content_trust.md).

### Prerequisites

These instructions assume you are running in Linux or Mac OS X. You can run
this sandbox on a local machine or on a virtual machine. You will need to
have `sudo` privileges on your local machine or in the VM.

This sandbox requires you to install two Docker tools: Docker Engine and Docker
Compose. To install the Docker Engine, choose from the [list of supported
platforms](../../installation). To install Docker Compose, see the
[detailed instructions here](https://docs.docker.com/compose/install/).

Finally, you'll need to have `git` installed on your local system or VM.

## What is in the sandbox?

If you are just using trust out-of-the-box you only need your Docker Engine
client and access to the Docker hub. The sandbox mimics a
production trust environment, and requires these additional components:

| Container       | Description                                                                                                                                 |
|-----------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| notarysandbox  | A container with the latest version of Docker Engine and with some preconfigured certifications. This is your sandbox where you can use the `docker` client to test trust operations. |
| Registry server | A local registry service.                                                                                                                 |
| Notary server   | The service that does all the heavy-lifting of managing trust                                                                               |
| Notary signer   | A service that ensures that your keys are secure.                                                                                           |
| MySQL           | The database where all of the trust information will be stored                                                                              |

The sandbox uses the Docker daemon on your local system. Within the `notarysandbox`
you interact with a local registry rather than the Docker Hub. This means
your everyday image repositories are not used. They are protected while you play.

When you play in the sandbox, you'll also create root and repository keys. The
sandbox is configured to store all the keys and files inside the `notarysandbox`
container. Since the keys you create in the sandbox are for play only,
destroying the container destroys them as well.


## Build the sandbox

In this section, you build the Docker components for your trust sandbox. If you
work exclusively with the Docker Hub, you would not need with these components.
They are built into the Docker Hub for you. For the sandbox, however, you must
build your own entire, mock production environment and registry.

### Configure /etc/hosts

The sandbox' `notaryserver` and `sandboxregistry` run on your local server. The
client inside the `notarysandbox` container connects to them over your network.
So, you'll need an entry for both the servers in your local `/etc/hosts` file.

1. Add an entry for the `notaryserver` to `/etc/hosts`.

        $ sudo sh -c 'echo "127.0.0.1 notaryserver" >> /etc/hosts'

2. Add an entry for the `sandboxregistry` to `/etc/hosts`.

        $ sudo sh -c 'echo "127.0.0.1 sandboxregistry" >> /etc/hosts'


### Build the notarytest image

1. Create a `notarytest` directory on your system.

        $ mkdir notarysandbox

2. Change into your `notarysandbox` directory.

        $ cd notarysandbox

3. Create a `notarytest` directory then change into that.

        $ mkdir notarytest
        $ cd notarytest

4. Create a filed called `Dockerfile` with your favorite editor.

5. Add the following to the new file.

        FROM debian:jessie

        ADD https://master.dockerproject.org/linux/amd64/docker /usr/bin/docker
        RUN chmod +x /usr/bin/docker \
          && apt-get update \
          && apt-get install -y \
          tree \
          vim \
          git \
          ca-certificates \
          --no-install-recommends

        WORKDIR /root
        RUN git clone -b trust-sandbox https://github.com/docker/notary.git
        RUN cp /root/notary/fixtures/root-ca.crt /usr/local/share/ca-certificates/root-ca.crt
        RUN update-ca-certificates

        ENTRYPOINT ["bash"]

6. Save and close the file.

7. Build the testing container.

        $ docker build -t notarysandbox .
        Sending build context to Docker daemon 2.048 kB
        Step 1 : FROM debian:jessie
         ...
         Successfully built 5683f17e9d72


### Build and start up the trust servers

In this step, you get the source code for your notary and registry services.
Then, you'll use Docker Compose to build and start them on your local system.

1. Change to back to the root of your  `notarysandbox` directory.

        $ cd notarysandbox

2. Clone the `notary` project.

          $ git clone -b trust-sandbox https://github.com/docker/notary.git

3. Clone the `distribution` project.

        $ git clone https://github.com/docker/distribution.git

4. Change to the `notary` project directory.

        $ cd notary

   The directory contains a `docker-compose` file that you'll use to run a
   notary server together with a notary signer and the corresponding MySQL
   databases. The databases store the trust information for an image.

5. Build the server images.

        $  docker-compose build

    The first time you run this, the build takes some time.

6. Run the server containers on your local system.

        $ docker-compose up -d

    Once the trust services are up, you'll setup a local version of the Docker
    Registry v2.

7. Change to the `notarysandbox/distribution` directory.

8. Build the `sandboxregistry` server.

        $ docker build -t sandboxregistry .

9. Start the `sandboxregistry` server running.

        $ docker run -p 5000:5000 --name sandboxregistry sandboxregistry &

## Playing in the sandbox

Now that everything is setup, you can go into your `notarysandbox` container and
start testing Docker content trust.


### Start the notarysandbox container

In this procedure, you start the `notarysandbox` and link it to the running
`notary_notaryserver_1` and `sandboxregistry` containers. The links allow
communication among the containers.

```
$ docker run -it -v /var/run/docker.sock:/var/run/docker.sock --link notary_notaryserver_1:notaryserver --link sandboxregistry:sandboxregistry notarysandbox
root@0710762bb59a:/#
```

Mounting the `docker.sock` gives the `notarysandbox` access to the `docker`
daemon on your host, while storing all the keys and files inside the sandbox
container.  When you destroy the container, you destroy the "play" keys.

### Test some trust operations

Now, you'll pull some images.

1. Download a `docker` image to test with.

        # docker pull docker/trusttest
        docker pull docker/trusttest
        Using default tag: latest
        latest: Pulling from docker/trusttest

        b3dbab3810fc: Pull complete
        a9539b34a6ab: Pull complete
        Digest: sha256:d149ab53f8718e987c3a3024bb8aa0e2caadf6c0328f1d9d850b2a2a67f2819a
        Status: Downloaded newer image for docker/trusttest:latest

2. Tag it to be pushed to our sandbox registry:

        # docker tag docker/trusttest sandboxregistry:5000/test/trusttest:latest

3. Enable content trust.

        # export DOCKER_CONTENT_TRUST=1

4. Identify the trust server.

        # export DOCKER_CONTENT_TRUST_SERVER=https://notaryserver:4443

    This step is only necessary because the sandbox is using its own server.
    Normally, if you are using the Docker Public Hub this step isn't necessary.

5. Pull the test image.

        # docker pull sandboxregistry:5000/test/trusttest
        Using default tag: latest
        no trust data available

      You see an error, because this content doesn't exist on the `sandboxregistry` yet.

6. Push the trusted image.

        # docker push sandboxregistry:5000/test/trusttest:latest
        The push refers to a repository [sandboxregistry:5000/test/trusttest] (len: 1)
        a9539b34a6ab: Image successfully pushed
        b3dbab3810fc: Image successfully pushed
        latest: digest: sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c size: 3348
        Signing and pushing trust metadata
        You are about to create a new root signing key passphrase. This passphrase
        will be used to protect the most sensitive key in your signing system. Please
        choose a long, complex passphrase and be careful to keep the password and the
        key file itself secure and backed up. It is highly recommended that you use a
        password manager to generate the passphrase and keep it safe. There will be no
        way to recover this key. You can find the key in your config directory.
        Enter passphrase for new root key with id 8c69e04:
        Repeat passphrase for new root key with id 8c69e04:
        Enter passphrase for new repository key with id sandboxregistry:5000/test/trusttest (93c362a):
        Repeat passphrase for new repository key with id sandboxregistry:5000/test/trusttest (93c362a):
        Finished initializing "sandboxregistry:5000/test/trusttest"
        latest: digest: sha256:d149ab53f8718e987c3a3024bb8aa0e2caadf6c0328f1d9d850b2a2a67f2819a size: 3355
        Signing and pushing trust metadata

7. Try pulling the image you just pushed:

        # docker pull sandboxregistry:5000/test/trusttest
        Using default tag: latest
        Pull (1 of 1): sandboxregistry:5000/test/trusttest:latest@sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c
        sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c: Pulling from test/trusttest
        b3dbab3810fc: Already exists
        a9539b34a6ab: Already exists
        Digest: sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c
        Status: Downloaded newer image for sandboxregistry:5000/test/trusttest@sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c
        Tagging sandboxregistry:5000/test/trusttest@sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c as sandboxregistry:5000/test/trusttest:latest


### Test with malicious images

What happens when data is corrupted and you try to pull it when trust is
enabled? In this section, you go into the `sandboxregistry` and tamper with some
data. Then, you try and pull it.

1. Leave the sandbox container running.

2. Open a new bash terminal from your host into the `sandboxregistry`.

        $ docker exec -it sandboxregistry bash
        296db6068327#

3. Change into the registry storage.

    You'll need to provide the `sha` you received when you pushed the image.

        # cd /var/lib/registry/docker/registry/v2/blobs/sha256/aa/aac0c133338db2b18ff054943cee3267fe50c75cdee969aed88b1992539ed042

4. Add malicious data to one of the trusttest layers:

        # echo "Malicious data" > data

5. Got back to your sandbox terminal.

6. List the trusttest image.

        # docker images | grep trusttest
        docker/trusttest                 latest              a9539b34a6ab        7 weeks ago         5.025 MB
        sandboxregistry:5000/test/trusttest   latest              a9539b34a6ab        7 weeks ago         5.025 MB
        sandboxregistry:5000/test/trusttest   <none>              a9539b34a6ab        7 weeks ago         5.025 MB

7. Remove the `trusttest:latest` image.

        # docker rmi -f a9539b34a6ab
        Untagged: docker/trusttest:latest
        Untagged: sandboxregistry:5000/test/trusttest:latest
        Untagged: sandboxregistry:5000/test/trusttest@sha256:1d871dcb16805f0604f10d31260e79c22070b35abc71a3d1e7ee54f1042c8c7c
        Deleted: a9539b34a6aba01d3942605dfe09ab821cd66abf3cf07755b0681f25ad81f675
        Deleted: b3dbab3810fc299c21f0894d39a7952b363f14520c2f3d13443c669b63b6aa20

8. Pull the image again.

        # docker pull sandboxregistry:5000/test/trusttest
        Using default tag: latest
        ...
        b3dbab3810fc: Verifying Checksum
        a9539b34a6ab: Pulling fs layer
        filesystem layer verification failed for digest sha256:aac0c133338db2b18ff054943cee3267fe50c75cdee969aed88b1992539ed042

      You'll see the the pull did not complete because the trust system was
      unable to verify the image.

## More play in the sandbox

Now, that you have a full Docker content trust sandbox on your local system,
feel free to play with it and see how it behaves. If you find any security
issues with Docker, feel free to send us an email at <security@docker.com>.


&nbsp;
