title
:   Building a Docker Image with MongoDB

description
:   How to build a Docker image with MongoDB pre-installed

keywords
:   docker, example, package installation, networking, mongodb

Building an Image with MongoDB
==============================

The goal of this example is to show how you can build your own Docker
images with MongoDB pre-installed. We will do that by constructing a
`Dockerfile` that downloads a base image, adds an apt source and
installs the database software on Ubuntu.

Creating a `Dockerfile`
-----------------------

Create an empty file called `Dockerfile`:

~~~~ {.sourceCode .bash}
touch Dockerfile
~~~~

Next, define the parent image you want to use to build your own image on
top of. Here, we’ll use [Ubuntu](https://index.docker.io/_/ubuntu/)
(tag: `latest`) available on the [docker index](http://index.docker.io):

~~~~ {.sourceCode .bash}
FROM    ubuntu:latest
~~~~

Since we want to be running the latest version of MongoDB we'll need to
add the 10gen repo to our apt sources list.

~~~~ {.sourceCode .bash}
# Add 10gen official apt source to the sources list
RUN apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 7F0CEB10
RUN echo 'deb http://downloads-distro.mongodb.org/repo/ubuntu-upstart dist 10gen' | tee /etc/apt/sources.list.d/10gen.list
~~~~

Then, we don't want Ubuntu to complain about init not being available so
we'll divert `/sbin/initctl` to `/bin/true` so it thinks everything is
working.

~~~~ {.sourceCode .bash}
# Hack for initctl not being available in Ubuntu
RUN dpkg-divert --local --rename --add /sbin/initctl
RUN ln -s /bin/true /sbin/initctl
~~~~

Afterwards we'll be able to update our apt repositories and install
MongoDB

~~~~ {.sourceCode .bash}
# Install MongoDB
RUN apt-get update
RUN apt-get install mongodb-10gen
~~~~

To run MongoDB we'll have to create the default data directory (because
we want it to run without needing to provide a special configuration
file)

~~~~ {.sourceCode .bash}
# Create the MongoDB data directory
RUN mkdir -p /data/db
~~~~

Finally, we'll expose the standard port that MongoDB runs on, 27107, as
well as define an `ENTRYPOINT` instruction for the container.

~~~~ {.sourceCode .bash}
EXPOSE 27017
ENTRYPOINT ["usr/bin/mongod"]
~~~~

Now, lets build the image which will go through the `Dockerfile` we made
and run all of the commands.

~~~~ {.sourceCode .bash}
sudo docker build -t <yourname>/mongodb .
~~~~

Now you should be able to run `mongod` as a daemon and be able to
connect on the local port!

~~~~ {.sourceCode .bash}
# Regular style
MONGO_ID=$(sudo docker run -d <yourname>/mongodb)

# Lean and mean
MONGO_ID=$(sudo docker run -d <yourname>/mongodb --noprealloc --smallfiles)

# Check the logs out
sudo docker logs $MONGO_ID

# Connect and play around
mongo --port <port you get from `docker ps`>
~~~~

Sweet!
