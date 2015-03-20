page_title: Working with Containers
page_description: Learn how to manage and operate Docker containers.
page_keywords: docker, the docker guide, documentation, docker.io, monitoring containers, docker top, docker inspect, docker port, ports, docker logs, log, Logs

# Working with Containers

In the [last section of the Docker User Guide](/userguide/dockerizing)
we launched our first containers. We launched two containers using the
`docker run` command.

* Containers we ran interactively in the foreground.
* One container we ran daemonized in the background.

In the process we learned about several Docker commands:

* `docker ps` - Lists containers.
* `docker logs` - Shows us the standard output of a container.
* `docker stop` - Stops running containers.

> **Tip:**
> Another way to learn about `docker` commands is our
> [interactive tutorial](https://www.docker.com/tryit/).

The `docker` client is pretty simple. Each action you can take
with Docker is a command and each command can take a series of
flags and arguments.

    # Usage:  [sudo] docker [command] [flags] [arguments] ..
    # Example:
    $ sudo docker run -i -t ubuntu /bin/bash

Let's see this in action by using the `docker version` command to return
version information on the currently installed Docker client and daemon.

    $ sudo docker version

This command will not only provide you the version of Docker client and
daemon you are using, but also the version of Go (the programming
language powering Docker).

    Client version: 0.8.0
    Go version (client): go1.2

    Git commit (client): cc3a8c8
    Server version: 0.8.0

    Git commit (server): cc3a8c8
    Go version (server): go1.2

    Last stable version: 0.8.0

### Seeing what the Docker client can do

We can see all of the commands available to us with the Docker client by
running the `docker` binary without any options.

    $ sudo docker

You will see a list of all currently available commands.

    Commands:
         attach    Attach to a running container
         build     Build an image from a Dockerfile
         commit    Create a new image from a container's changes
    . . .

### Seeing Docker command usage

You can also zoom in and review the usage for specific Docker commands.

Try typing Docker followed with a `[command]` to see the usage for that
command:

    $ sudo docker attach
    Help output . . .

Or you can also pass the `--help` flag to the `docker` binary.

    $ sudo docker attach --help

This will display the help text and all available flags:

    Usage: docker attach [OPTIONS] CONTAINER

    Attach to a running container

      --no-stdin=false: Do not attach stdin
      --sig-proxy=true: Proxify all received signal to the process (non-TTY mode only)

> **Note:** 
> You can see a full list of Docker's commands
> [here](/reference/commandline/cli/).

## Running a Web Application in Docker

So now we've learnt a bit more about the `docker` client let's move onto
the important stuff: running more containers. So far none of the
containers we've run did anything particularly useful though. So let's
build on that experience by running an example web application in
Docker.

For our web application we're going to run a Python Flask application.
Let's start with a `docker run` command.

    $ sudo docker run -d -P training/webapp python app.py

Let's review what our command did. We've specified two flags: `-d` and
`-P`. We've already seen the `-d` flag which tells Docker to run the
container in the background. The `-P` flag is new and tells Docker to
map any required network ports inside our container to our host. This
lets us view our web application.

We've specified an image: `training/webapp`. This image is a
pre-built image we've created that contains a simple Python Flask web
application.

Lastly, we've specified a command for our container to run: `python app.py`. This launches our web application.

> **Note:** 
> You can see more detail on the `docker run` command in the [command
> reference](/reference/commandline/cli/#run) and the [Docker Run
> Reference](/reference/run/).

## Viewing our Web Application Container

Now let's see our running container using the `docker ps` command.

    $ sudo docker ps -l
    CONTAINER ID  IMAGE                   COMMAND       CREATED        STATUS        PORTS                    NAMES
    bc533791f3f5  training/webapp:latest  python app.py 5 seconds ago  Up 2 seconds  0.0.0.0:49155->5000/tcp  nostalgic_morse

You can see we've specified a new flag, `-l`, for the `docker ps`
command. This tells the `docker ps` command to return the details of the
*last* container started.

> **Note:** 
> By default, the `docker ps` command only shows information about running
> containers. If you want to see stopped containers too use the `-a` flag.

We can see the same details we saw [when we first Dockerized a
container](/userguide/dockerizing) with one important addition in the `PORTS`
column.

    PORTS
    0.0.0.0:49155->5000/tcp

When we passed the `-P` flag to the `docker run` command Docker mapped any
ports exposed in our image to our host.

> **Note:** 
> We'll learn more about how to expose ports in Docker images when
> [we learn how to build images](/userguide/dockerimages).

In this case Docker has exposed port 5000 (the default Python Flask
port) on port 49155.

Network port bindings are very configurable in Docker. In our last example the
`-P` flag is a shortcut for `-p 5000` that maps port 5000 inside the container
to a high port (from *ephemeral port range* which typically ranges from 32768
to 61000) on the local Docker host. We can also bind Docker containers to
specific ports using the `-p` flag, for example:

    $ sudo docker run -d -p 5000:5000 training/webapp python app.py

This would map port 5000 inside our container to port 5000 on our local
host. You might be asking about now: why wouldn't we just want to always
use 1:1 port mappings in Docker containers rather than mapping to high
ports? Well 1:1 mappings have the constraint of only being able to map
one of each port on your local host. Let's say you want to test two
Python applications: both bound to port 5000 inside their own containers.
Without Docker's port mapping you could only access one at a time on the
Docker host.

So let's now browse to port 49155 in a web browser to
see the application.

![Viewing the web application](/userguide/webapp1.png).

Our Python application is live!

> **Note:**
> If you have used the boot2docker virtual machine on OS X, Windows or Linux,
> you'll need to get the IP of the virtual host instead of using localhost.
> You can do this by running the following in
> the boot2docker shell.
> 
>     $ boot2docker ip
>     The VM's Host only interface IP address is: 192.168.59.103
> 
> In this case you'd browse to http://192.168.59.103:49155 for the above example.

## A Network Port Shortcut

Using the `docker ps` command to return the mapped port is a bit clumsy so
Docker has a useful shortcut we can use: `docker port`. To use `docker port` we
specify the ID or name of our container and then the port for which we need the
corresponding public-facing port.

    $ sudo docker port nostalgic_morse 5000
    0.0.0.0:49155

In this case we've looked up what port is mapped externally to port 5000 inside
the container.

## Viewing the Web Application's Logs

Let's also find out a bit more about what's happening with our application and
use another of the commands we've learnt, `docker logs`.

    $ sudo docker logs -f nostalgic_morse
    * Running on http://0.0.0.0:5000/
    10.0.2.2 - - [23/May/2014 20:16:31] "GET / HTTP/1.1" 200 -
    10.0.2.2 - - [23/May/2014 20:16:31] "GET /favicon.ico HTTP/1.1" 404 -

This time though we've added a new flag, `-f`. This causes the `docker
logs` command to act like the `tail -f` command and watch the
container's standard out. We can see here the logs from Flask showing
the application running on port 5000 and the access log entries for it.

## Looking at our Web Application Container's processes

In addition to the container's logs we can also examine the processes
running inside it using the `docker top` command.

    $ sudo docker top nostalgic_morse
    PID                 USER                COMMAND
    854                 root                python app.py

Here we can see our `python app.py` command is the only process running inside
the container.

## Inspecting our Web Application Container

Lastly, we can take a low-level dive into our Docker container using the
`docker inspect` command. It returns a JSON hash of useful configuration
and status information about Docker containers.

    $ sudo docker inspect nostalgic_morse

Let's see a sample of that JSON output.

    [{
        "ID": "bc533791f3f500b280a9626688bc79e342e3ea0d528efe3a86a51ecb28ea20",
        "Created": "2014-05-26T05:52:40.808952951Z",
        "Path": "python",
        "Args": [
           "app.py"
        ],
        "Config": {
           "Hostname": "bc533791f3f5",
           "Domainname": "",
           "User": "",
    . . .

We can also narrow down the information we want to return by requesting a
specific element, for example to return the container's IP address we would:

    $ sudo docker inspect -f '{{ .NetworkSettings.IPAddress }}' nostalgic_morse
    172.17.0.5

## Stopping our Web Application Container

Okay we've seen web application working. Now let's stop it using the
`docker stop` command and the name of our container: `nostalgic_morse`.

    $ sudo docker stop nostalgic_morse
    nostalgic_morse

We can now use the `docker ps` command to check if the container has
been stopped.

    $ sudo docker ps -l

## Restarting our Web Application Container

Oops! Just after you stopped the container you get a call to say another
developer needs the container back. From here you have two choices: you
can create a new container or restart the old one. Let's look at
starting our previous container back up.

    $ sudo docker start nostalgic_morse
    nostalgic_morse

Now quickly run `docker ps -l` again to see the running container is
back up or browse to the container's URL to see if the application
responds.

> **Note:** 
> Also available is the `docker restart` command that runs a stop and
> then start on the container.

## Removing our Web Application Container

Your colleague has let you know that they've now finished with the container
and won't need it again. So let's remove it using the `docker rm` command.

    $ sudo docker rm nostalgic_morse
    Error: Impossible to remove a running container, please stop it first or use -f
    2014/05/24 08:12:56 Error: failed to remove one or more containers

What's happened? We can't actually remove a running container. This protects
you from accidentally removing a running container you might need. Let's try
this again by stopping the container first.

    $ sudo docker stop nostalgic_morse
    nostalgic_morse
    $ sudo docker rm nostalgic_morse
    nostalgic_morse

And now our container is stopped and deleted.

> **Note:**
> Always remember that deleting a container is final!

# Next steps

Until now we've only used images that we've downloaded from
[Docker Hub](https://hub.docker.com) now let's get introduced to
building and sharing our own images.

Go to [Working with Docker Images](/userguide/dockerimages).

