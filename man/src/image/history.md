Show the history of when and how an image was created.

# EXAMPLES
    $ docker history fedora
    IMAGE          CREATED          CREATED BY                                      SIZE                COMMENT
    105182bb5e8b   5 days ago       /bin/sh -c #(nop) ADD file:71356d2ad59aa3119d   372.7 MB
    73bd853d2ea5   13 days ago      /bin/sh -c #(nop) MAINTAINER Lokesh Mandvekar   0 B
    511136ea3c5a   10 months ago                                                    0 B                 Imported from -

## Display comments in the image history
The `docker commit` command has a **-m** flag for adding comments to the image. These comments will be displayed in the image history.

    $ sudo docker history docker:scm
    IMAGE               CREATED             CREATED BY                                      SIZE                COMMENT
    2ac9d1098bf1        3 months ago        /bin/bash                                       241.4 MB            Added Apache to Fedora base image
    88b42ffd1f7c        5 months ago        /bin/sh -c #(nop) ADD file:1fd8d7f9f6557cafc7   373.7 MB            
    c69cab00d6ef        5 months ago        /bin/sh -c #(nop) MAINTAINER Lokesh Mandvekar   0 B                 
    511136ea3c5a        19 months ago                                                       0 B                 Imported from -
