Docker concepts
===============

Image
-----

An image is a root filesystem + some metadata. It is uniquely identified by a SHA256, and it can be given a symbolic name as well (so you can `docker run mystuff` instead of `docker run 819f04e5706f5...`.

The metadata is a JSON blob. It contains at least:

- the hash of the parent (only if the image is based on another image; if it was created from scratch, then the parent is `null`),
- the creation date (this is when the `docker commit` command was done).

The hash of the image is defined as:

`SHA256(SHA256(jsondata)+SHA256(tarball))`

When you run something into an image, it automatically creates a container. The container has a unique ID, and when you `docker commit <container_id> mystuff`, you are creating a new image, and giving it the nickname `mystuff`.


Repository
----------

A repository:

- belongs to a specific user,
- has a given name chosen by the user,
- is a set of tagged images.

The typical use case is to group different versions of something under a repository.

Example: you are John Doe, maintainer of a collection of PostgreSQL images based on different releases of Ubuntu. Your docker ID is `jdoe`; you decide that the repository name will by `pgsql`. You pull a bunch of base images for the different Ubuntu releases, then you setup different versions of PostgreSQL in them. You end up with the following set of images:

- a base lucid image,
- a base precise image,
- a base quantal image,
- PostgreSQL 9.1 installed on top of the lucid image,
- PostgreSQL 9.2 installed on top of the lucid image,
- PostgreSQL 9.1 installed on top of the precise image,
- PostgreSQL 9.2 installed on top of the precise image,
- PostgreSQL 9.1 installed on top of the quantal image,
- PostgreSQL 9.2 installed on top of the quantal image,
- PostgreSQL 9.3 installed on top of the quantal image.

The first three won't be in the repository, but the other ones will. You decide that the tags will be lucid9.1, lucid9.2, precise9.1, etc.

Note: those images do not have to share a common ancestor. In this case, we have three "root" images (one for each base Ubuntu release).

When someone wants to use one of your images, he will do something like:

    docker run -p 5432 jdoe/pgsql@lucid9.2 postgres -D /var/lib/...

Docker will do the following:

- notice that the image name contains a slash, and is therefore a reference to a repository;
- notice that the image name contains an arroba, and is therefore a reference to a specific version;
- query the docker registry to resolve jdoe/pgsql@lucid9.2 into an image ID;
- download the image metadata+tarball from the registry (unless it already has them locally);
- recursively download all the parent images of the image (unless it already has them locally);
- run the image.

There is one special version: `latest`. When you don't request a specific version, you are implying that you want the `latest` version. When you push a version (any version!) to the repository, you are also pushing to `latest` as well.

QUESTION: do we want to update `latest` even if the commit date of the image is older than the current `latest` image?

QUESTION: who should update `latest`? Should it be done by the docker client, or automatically done server-side?



Confused?
---------

Another way to put it: a "repository" is like the "download binaries" page for a given product of a software vendor. Once version 1.42.5 is there, it probably won't be modified (they will rather release 1.42.6), unless there was something really harmful or embarrassing in 1.42.5.


Storage of images
-----------------

Images are to be stored on S3.

A given image will be mapped to two S3 objects:

- s3://get.docker.io/images/<id>/json (plain JSON file)
- s3://get.docker.io/images/<id>/layer (tarball)

The S3 storage is authoritative. I.E. the registry will very probably keep some cache of the metadata, but it will be just a cache.


Storage of repositories
-----------------------

TBD


Pull images
-----------

Pulling an image is fairly straightforward:

    GET /v1/images/<id>/json
    GET /v1/images/<id>/layer
    GET /v1/images/<id>/history

The first two calls redirect you to their S3 counterparts. But before redirecting you, the registry checks (probably with `HEAD` requests) that both `json` and `layer` objects actually exist on S3. I.E., if there was a partial upload, when you try to `GET` the `json` or the `layer` object, the registry will give you a 404 for both objects, even if one of them does exist.

The last one sends you a JSON payload, which is a list containing all the metadata of the image and all its ancestors. The requested image comes first, then its parent, then the parent of the parent, etc.

SUGGESTION: rename `history` to `ancestry` (it sounds more hipstery, but it's actually more accurate)

SUGGESTION: add optional parameter `?length=X` to `history`, so you can limit to `X` ancestors, and avoid pulling 42000 ancestors in one go - especially if you have most of them already...


Push images
-----------

The first thing is to push the meta data:

    PUT /v1/images/<id>/json

Four things can happen:

- invalid/empty JSON: the server tells you to go away (HTTP 400?)
- image already exists with the same JSON: the server tells you that it's fine (HTTP 204?)
- image already exists but is different: the server informs you that something's wrong (?)
- image doesn't exist: the server puts the JSON on S3, then generates an upload URL for the tarball, and sends you an HTTP 200 containing this upload URL

In the latter case, you want to move to the next step:

    PUT the tarball to whatever-URL-you-got-on-previous-stage

SUGGESTION: consider a `PUT /v1/images/<id>/layer` with `Except: 100-continue` and honor a 301/302 redirect. This might or might not be legal HTTP.

The last thing is to try to push the parent image (unless you're sure that it is already in the registry). If the image is already there, stop. If it's not there, upload it, and recursively upload its parents in a similar fashion.


Pull repository
---------------

This:

    GET /v1/users/<userid>/<reponame>

Sends back a JSON dict mapping version tags to image version, e.g.:

    { 
       "1.1": "87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7",
       "1.2": "0263829989b6fd954f72baaf2fc64bc2e2f01d692d4de72986ea808f6e99813f",
       "latest": "0263829989b6fd954f72baaf2fc64bc2e2f01d692d4de72986ea808f6e99813f"
    }

SUGGESTION: also allow this URL:

    GET /v1/users/<userid>/<reponame>/<versiontag>

Which would send back the image version hash.


Push repository
---------------

This:

    PUT /v1/users/<userid>/<reponame>/<versiontag>

The request body should be the image version hash.


Example session
---------------

First idea:

    # Automatically pull base, aka docker/base@latest, and run something in it
    docker run base ...
    (Output: 42424242)
    docker commit 42424242 databeze
    docker login jdoe s3s4me!
    # The following two commands are equivalent
    docker push jdoe/pgsql databeze
    docker push jdoe/pgsql 42424242

Second idea:

    docker run base ...
    docker commit 42424242 jdoe/pgsql
    docker login jdoe s3s4me!
    docker push jdoe/pgsql

Maybe this would work too:

    docker commit 42424242 pgsql
    docker push pgsql

And maybe this too:

    docker push -a

NOTE: when your commit overwrites an existing tag, the image should be marked "dirty" so that docker knows that it has to be pushed.

NOTE: if a pull would cause some local tag to be overwritten, docker could refuse, and ask you to rename your local tag, or ask you to specify a -f flag to overwrite. Your local changes won't be lost, but the tag will be lost, so if yon don't know the image ID it could be hard to figure out which one it was.

NOTE: we probably need some commands to move/remove tags to images.

Collaborative workflow:

    alice# docker login mybigco p455w0rd
    bob# docker login mybigco p455w0rd
    alice# docker pull base
    alice# docker run -a -t -i base /bin/sh
    ... hard core authoring takes place ...
    alice# docker commit <container_id> wwwbigco
    alice# docker push wwwbigco
    ... the latter actually does docker push mybigco/wwwbigco@latest ...
    bob# docker pull mybigco/wwwbigco
    bob# docker run mybigco/wwwbigcom /usr/sbin/nginx
    ... change things ...
    bob# docker commit <container_id> wwwbigco
    bob# docker push wwwbigco

NOTE: what about this?
