% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2015
# NAME
docker-squash - Merge filesystem layers of an image into a new image

# SYNOPSIS
**docker squash**
[**--help**]
[**--no-trunc**[=*false*]]
[**-t**|**--tag**[=*TAG*]]
IMAGE [ANCESTOR]

# DESCRIPTION

This command combines the filesystem layers between an image and an ancestor
image (or all layers if no ancestor is specified) into a new image.

# OPTIONS
**--help**
  Print usage statement

**--no-trunc**=*true*|*false*
   Don't truncate output. The default is *false*.

**-t**, **--tag**=""
   Repository name (and optionally a tag) to be applied to the resulting image
   in case of success.

# EXAMPLES

# Consolidating all layer of an image to a new single-layered image

Let's inspect the history of the busybox image.

    # docker history busybox
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    8c2e06607696        8 weeks ago         /bin/sh -c #(nop) CMD ["/bin/sh"]               0 B                 
    6ce2e90b0bc7        8 weeks ago         /bin/sh -c #(nop) ADD file:8cf517d90fe79547c4   2.43 MB             
    cf2616975b4a        8 weeks ago         /bin/sh -c #(nop) MAINTAINER Jérôme Petazzo     0 B

It has 3 layers total, but we can now `squash` it and tag it as something else.

    # docker squash busybox
    ccbe958582b2
    # docker tag ccbe958582b2 better_busybox

Now we have these 2 images.

    # docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED              VIRTUAL SIZE
    better_busybox      latest              ccbe958582b2        About a minute ago   2.43 MB
    busybox             latest              8c2e06607696        8 weeks ago          2.43 MB

But `better_busybox` only has a single layer in its history.

    # docker history better_busybox
    IMAGE               CREATED              CREATED BY          SIZE                COMMENT
    ccbe958582b2        About a minute ago                       2.43 MB 


# Merging layers of a newly built image into a single layer relative to the
# base image.

Let's build an image using this Dockerfile:


    FROM busybox:latest
    MAINTAINER Josh Hawn <josh.hawn@docker.com>
    ADD docker/docs /docs
    RUN echo hello > /hello.txt

    # docker build -t test_build .
    Sending build context to Docker daemon 93.87 MB
    Step 0 : FROM busybox:latest
     ---> 8c2e06607696
    Step 1 : MAINTAINER Josh Hawn <josh.hawn@docker.com>
     ---> Running in 0c2b29e087d5
     ---> 4ac4b75a88f6
    Removing intermediate container 0c2b29e087d5
    Step 2 : ADD docker/docs /docs
     ---> c9e50d91bc69
    Removing intermediate container ad9e921d943f
    Step 3 : RUN echo hello > /hello.txt
     ---> Running in e15283387c37
     ---> 49644b91f444
    Removing intermediate container e15283387c37
    Successfully built 49644b91f444

And inspect the history of this new image:

    # docker history test_build
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    49644b91f444        24 minutes ago      /bin/sh -c echo hello > /hello.txt              6 B                 
    c9e50d91bc69        24 minutes ago      /bin/sh -c #(nop) ADD dir:24ae8a736955084e39d   8.537 MB            
    4ac4b75a88f6        24 minutes ago      /bin/sh -c #(nop) MAINTAINER Josh Hawn <josh.   0 B                 
    8c2e06607696        8 weeks ago         /bin/sh -c #(nop) CMD ["/bin/sh"]               0 B                 
    6ce2e90b0bc7        8 weeks ago         /bin/sh -c #(nop) ADD file:8cf517d90fe79547c4   2.43 MB             
    cf2616975b4a        8 weeks ago         /bin/sh -c #(nop) MAINTAINER Jérôme Petazzo     0 B

See that it has 6 layers. That's 3 more than were already in the base image. We
can squash these 3 into an image with one additional layer relative to the base
image, and re-tag this new image.

    # docker squash test_build busybox
    97561533b0fa
    # docker tag -f 97561533b0fa test_build

Now `test_build` only has a single new layer on top of the original 3 layers
from `busybox`:

    # docker history test_build
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    97561533b0fa        2 minutes ago                                                       8.537 MB            
    8c2e06607696        8 weeks ago         /bin/sh -c #(nop) CMD ["/bin/sh"]               0 B                 
    6ce2e90b0bc7        8 weeks ago         /bin/sh -c #(nop) ADD file:8cf517d90fe79547c4   2.43 MB             
    cf2616975b4a        8 weeks ago         /bin/sh -c #(nop) MAINTAINER Jérôme Petazzo     0 B

# HISTORY
June 2015, Originally compiled by Josh Hawn

