page_title: Docker Hub Enterprise: User guide
page_description: Documentation describing basic use of Docker Hub Enterprise
page_keywords: docker, documentation, about, technology, hub, enterprise


# Docker Hub Enterprise User's Guide

This guide covers tasks and functions a user of Docker Hub Enterprise (DHE) will
need to know about, such as pushing or pulling images, etc. For tasks DHE
administrators need to accomplish, such as configuring or monitoring DHE, please
visit the [Administrator's Guide](./adminguide.md).

## Using DHE to push and pull images

The primary use case for DHE users is to push and pull images to and from the
DHE image storage service. The following instructions describe these procedures.

> **Note**: If your DHE instance has authentication enabled, you will need to
>use your command line to `docker login <dhe-hostname>` (e.g., `docker login
> dhe.yourdomain.com`).
>
> Failures due to unauthenticated `docker push` and `docker pull` commands will
> look like :
>
>     $ docker pull dhe.yourdomain.com/hello-world
>     Pulling repository dhe.yourdomain.com/hello-world
>     FATA[0001] Error: image hello-world:latest not found
>
>     $ docker push dhe.yourdomain.com/hello-world
>     The push refers to a repository [dhe.yourdomain.com/hello-world] (len: 1)
>     e45a5af57b00: Image push failed
>     FATA[0001] Error pushing to registry: token auth attempt for registry https://dhe.yourdomain.com/v2/: https://>     dhe.yourdomain.com/auth/v2/token/?scope=repository%3Ahello-world%3Apull%2Cpush&service=dhe.yourdomain.com >     request failed with status: 401 Unauthorized


1. Pull the `hello-world` official image from the Docker Hub. By default, if
Docker can't find an image locally, it will attempt to pull the image from the
Docker Hub.

    `$ docker pull hello-world`

2. List your available images.

        $ docker images
        REPOSITORY     TAG     IMAGE ID      CREATED       VIRTUAL SIZE
        hello-world    latest  e45a5af57b00  3 months ago  910 B

    Your list should include the `hello-world` image from the earlier run.

3. Re-tag the `hello-world` image so that it refers to your DHE server.

    `$ docker tag hello-world:latest dhe.yourdomain.com/demouser/hello-mine:latest`

    The command labels a `hello-world:latest` image using a new tag in the
    `[REGISTRYHOST/][USERNAME/]NAME[:TAG]` format.  The `REGISTRYHOST` in this
    case is the DHE server, `dhe.yourdomain.com`, and the `USERNAME` is
    `demouser`.

4. List your new image.

        $ docker images
        REPOSITORY                           TAG     IMAGE ID      CREATED       VIRTUAL SIZE
        hello-world                          latest  e45a5af57b00  3 months ago  910 B
        dhe.yourdomain.com/demouser/hello-mine  latest  e45a5af57b00  3 months ago  910 B

    You should see your new image label in the listing, with the same `IMAGE ID`
    as the Official image.

5. Push this new image to your DHE server.

    `$ docker push dhe.yourdomain.com/demouser/hello-mine:latest`

6. Set up a test of DHE by removing all images from your local environment:

    `$ docker rmi -f $(docker images -q -a)`

    This command is for illustrative purposes only: removing the image forces
    any subsequent `run` to pull from a remote registry (such as DHE) rather
    than from a local cache. If you run `docker images` after this you should
    not see any instance of `hello-world` or `hello-mine` in your images list.

        $ docker images
        REPOSITORY      TAG      IMAGE ID      CREATED       VIRTUAL SIZE

7. Try running `hello-mine`.

        $ docker run hello-mine
        Unable to find image 'hello-mine:latest' locally
        Pulling repository hello-mine
        FATA[0007] Error: image library/hello-mine:latest not found

    The `run` command fails because your new image doesn't exist on the Docker Hub.

8. Run `hello-mine` again, this time pointing it to pull from DHE:

        $ docker run dhe.yourdomain.com/demouser/hello-mine
        latest: Pulling from dhe.yourdomain.com/demouser/hello-mine
        511136ea3c5a: Pull complete
        31cbccb51277: Pull complete
        e45a5af57b00: Already exists
        Digest: sha256:45f0de377f861694517a1440c74aa32eecc3295ea803261d62f950b1b757bed1
        Status: Downloaded newer image for dhe.yourdomain.com/demouser/hello-mine:latest

    If you run `docker images` after this you'll see a `hello-mine` image.

        $ docker images
        REPOSITORY                           TAG     IMAGE ID      CREATED       VIRTUAL SIZE
        dhe.yourdomain.com/demouser/hello-mine  latest  e45a5af57b00  3 months ago  910 B

> **Note**: If the Docker daemon on which you are running `docker push` doesn't
> have the right certificates set up, you will get an error similar to:
>
>     $ docker push dhe.yourdomain.com/demouser/hello-world
>     FATA[0000] Error response from daemon: v1 ping attempt failed with error: Get https://dhe.yourdomain.com/v1/_ping: x509: certificate signed by unknown authority. If this private registry supports only HTTP or HTTPS with an unknown CA certificate, please add `--insecure-registry dhe.yourdomain.com` to the daemon's arguments. In the case of HTTPS, if you have access to the registry's CA certificate, no need for the flag; simply place the CA certificate at /etc/docker/certs.d/dhe.yourdomain.com/ca.crt

9. You have now successfully created a custom image, `hello-mine`, tagged it,
    and pushed it to the image storage provided by your DHE instance. You then
    pulled that image back down from DHE and onto your machine, where you can
    use it to create a container containing the "Hello World" application..

## Next Steps

For information on administering DHE, take a look at the [Administrator's Guide](./adminguide.md).


<!--TODO:

* mention that image aliases that are not in the same repository are not updated - either on push or pull
* but that multiple tags in one repo are pushed if you don't specify the `:tag` (ie, `imagename` does not always mean `imagename:latest`)
* show what happens for non-latest, and when there are more than one tag in a repo
* explain the fully-qualified repo/image name -->
