page_title: Linking Containers Together
page_description: Learn how to connect Docker containers together.
page_keywords: Examples, Usage, user guide, links, linking, docker, documentation, examples, names, name, container naming, port, map, network port, network

# Linking Containers Together

In [the Using Docker section](/userguide/usingdocker) we touched on
connecting to a service running inside a Docker container via a network
port. This is one of the ways that you can interact with services and
applications running inside Docker containers. In this section we're
going to give you a refresher on connecting to a Docker container via a
network port as well as introduce you to the concepts of container
linking.

## Network port mapping refresher

In [the Using Docker section](/userguide/usingdocker) we created a
container that ran a Python Flask application.

    $ sudo docker run -d -P training/webapp python app.py

> **Note:** 
> Containers have an internal network and an IP address
> (remember we used the `docker inspect` command to show the container's
> IP address in the [Using Docker](/userguide/usingdocker/) section).
> Docker can have a variety of network configurations. You can see more
> information on Docker networking [here](/articles/networking/).

When we created that container we used the `-P` flag to automatically map any
network ports inside that container to a random high port from the range 49000
to 49900 on our Docker host.  When we subsequently ran `docker ps` we saw that
port 5000 was bound to port 49155.

    $ sudo docker ps nostalgic_morse
    CONTAINER ID  IMAGE                   COMMAND       CREATED        STATUS        PORTS                    NAMES
    bc533791f3f5  training/webapp:latest  python app.py 5 seconds ago  Up 2 seconds  0.0.0.0:49155->5000/tcp  nostalgic_morse

We also saw how we can bind a container's ports to a specific port using
the `-p` flag.

    $ sudo docker run -d -p 5000:5000 training/webapp python app.py

And we saw why this isn't such a great idea because it constrains us to
only one container on that specific port.

There are also a few other ways we can configure the `-p` flag. By
default the `-p` flag will bind the specified port to all interfaces on
the host machine. But we can also specify a binding to a specific
interface, for example only to the `localhost`.

    $ sudo docker run -d -p 127.0.0.1:5000:5000 training/webapp python app.py

This would bind port 5000 inside the container to port 5000 on the
`localhost` or `127.0.0.1` interface on the host machine.

Or to bind port 5000 of the container to a dynamic port but only on the
`localhost` we could:

    $ sudo docker run -d -p 127.0.0.1::5000 training/webapp python app.py

We can also bind UDP ports by adding a trailing `/udp`, for example:

    $ sudo docker run -d -p 127.0.0.1:5000:5000/udp training/webapp python app.py

We also saw the useful `docker port` shortcut which showed us the
current port bindings, this is also useful for showing us specific port
configurations. For example if we've bound the container port to the
`localhost` on the host machine this will be shown in the `docker port`
output.

    $ docker port nostalgic_morse
    127.0.0.1:49155

> **Note:** 
> The `-p` flag can be used multiple times to configure multiple ports.

## Docker Container Linking

Network port mappings are not the only way Docker containers can connect
to one another. Docker also has a linking system that allows you to link
multiple containers together and share connection information between
them. Docker linking will create a parent child relationship where the
parent container can see selected information about its child.

## Container naming

To perform this linking Docker relies on the names of your containers.
We've already seen that each container we create has an automatically
created name, indeed we've become familiar with our old friend
`nostalgic_morse` during this guide. You can also name containers
yourself. This naming provides two useful functions:

1. It's useful to name containers that do specific functions in a way
   that makes it easier for you to remember them, for example naming a
   container with a web application in it `web`.

2. It provides Docker with reference point that allows it to refer to other
   containers, for example link container `web` to container `db`.

You can name your container by using the `--name` flag, for example:

    $ sudo docker run -d -P --name web training/webapp python app.py

You can see we've launched a new container and used the `--name` flag to
call the container `web`. We can see the container's name using the
`docker ps` command.

    $ sudo docker ps -l
    CONTAINER ID  IMAGE                  COMMAND        CREATED       STATUS       PORTS                    NAMES
    aed84ee21bde  training/webapp:latest python app.py  12 hours ago  Up 2 seconds 0.0.0.0:49154->5000/tcp  web

We can also use `docker inspect` to return the container's name.

    $ sudo docker inspect -f "{{ .Name }}" aed84ee21bde
    /web

> **Note:** 
> Container names have to be unique. That means you can only call
> one container `web`. If you want to re-use a container name you must delete the
> old container with the `docker rm` command before you can create a new
> container with the same name. As an alternative you can use the `--rm`
> flag with the `docker run` command. This will delete the container
> immediately after it stops.

## Container Linking

Links allow containers to discover and securely communicate with each
other. To create a link you use the `--link` flag. Let's create a new
container, this one a database.

    $ sudo docker run -d --name db training/postgres

Here we've created a new container called `db` using the `training/postgres`
image, which contains a PostgreSQL database.

Now let's create a new `web` container and link it with our `db` container.

    $ sudo docker run -d -P --name web --link db:db training/webapp python app.py

This will link the new `web` container with the `db` container we created
earlier. The `--link` flag takes the form:

    --link name:alias

Where `name` is the name of the container we're linking to and `alias` is an
alias for the link name. We'll see how that alias gets used shortly.

Let's look at our linked containers using `docker ps`.

    $ docker ps
    CONTAINER ID  IMAGE                     COMMAND               CREATED             STATUS             PORTS                    NAMES
    349169744e49  training/postgres:latest  su postgres -c '/usr  About a minute ago  Up About a minute  5432/tcp                 db
    aed84ee21bde  training/webapp:latest    python app.py         16 hours ago        Up 2 minutes       0.0.0.0:49154->5000/tcp  db/web,web

We can see our named containers, `db` and `web`, and we can see that the `web`
containers also shows `db/web` in the `NAMES` column. This tells us that the
`web` container is linked to the `db` container in a parent/child relationship.

So what does linking the containers do? Well we've discovered the link creates
a parent-child relationship between the two containers. The parent container,
here `db`, can access information on the child container `web`. To do this
Docker creates a secure tunnel between the containers without the need to
expose any ports externally on the container. You'll note when we started the
`db` container we did not use either of the `-P` or `-p` flags. As we're
linking the containers we don't need to expose the PostgreSQL database via the
network.

Docker exposes connectivity information for the parent container inside the
child container in two ways:

* Environment variables,
* Updating the `/etc/host` file.

Let's look first at the environment variables Docker sets. Inside the `web`
container let's run the `env` command to list the container's environment
variables.

    root@aed84ee21bde:/opt/webapp# env
    HOSTNAME=aed84ee21bde
    . . .
    DB_NAME=/web/db
    DB_PORT=tcp://172.17.0.5:5432
    DB_PORT_5000_TCP=tcp://172.17.0.5:5432
    DB_PORT_5000_TCP_PROTO=tcp
    DB_PORT_5000_TCP_PORT=5432
    DB_PORT_5000_TCP_ADDR=172.17.0.5
    . . .

> **Note**:
> These Environment variables are only set for the first process in the
> container. Similarly, some daemons (such as `sshd`)
> will scrub them when spawning shells for connection.

We can see that Docker has created a series of environment variables with
useful information about our `db` container. Each variables is prefixed with
`DB` which is populated from the `alias` we specified above. If our `alias`
were `db1` the variables would be prefixed with `DB1_`. You can use these
environment variables to configure your applications to connect to the database
on the `db` container. The connection will be secure, private and only the
linked `web` container will be able to talk to the `db` container.

In addition to the environment variables Docker adds a host entry for the
linked parent to the `/etc/hosts` file. Let's look at this file on the `web`
container now.

    root@aed84ee21bde:/opt/webapp# cat /etc/hosts
    172.17.0.7  aed84ee21bde
    . . .
    172.17.0.5  db

We can see two relevant host entries. The first is an entry for the `web`
container that uses the Container ID as a host name. The second entry uses the
link alias to reference the IP address of the `db` container. Let's try to ping
that host now via this host name.

    root@aed84ee21bde:/opt/webapp# apt-get install -yqq inetutils-ping
    root@aed84ee21bde:/opt/webapp# ping db
    PING db (172.17.0.5): 48 data bytes
    56 bytes from 172.17.0.5: icmp_seq=0 ttl=64 time=0.267 ms
    56 bytes from 172.17.0.5: icmp_seq=1 ttl=64 time=0.250 ms
    56 bytes from 172.17.0.5: icmp_seq=2 ttl=64 time=0.256 ms

> **Note:** 
> We had to install `ping` because our container didn't have it.

We've used the `ping` command to ping the `db` container using it's host entry
which resolves to `172.17.0.5`. We can make use of this host entry to configure
an application to make use of our `db` container.

> **Note:** 
> You can link multiple child containers to a single parent. For
> example, we could have multiple web containers attached to our `db`
> container.

# Next step

Now we know how to link Docker containers together the next step is
learning how to manage data, volumes and mounts inside our containers.

Go to [Managing Data in Containers](/userguide/dockervolumes).

