page_title: Command Line Interface
page_description: Docker's CLI command description and usage
page_keywords: Docker, Docker documentation, CLI, command line

# Command Line

To list available commands, either run `docker` with no parameters
or execute `docker help`:

    $ sudo docker
      Usage: docker [OPTIONS] COMMAND [arg...]
        -H, --host=[]: The socket(s) to bind to in daemon mode, specified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.

      A self-sufficient runtime for linux containers.

      ...

## Option types

Single character commandline options can be combined, so rather than
typing `docker run -t -i --name test busybox sh`,
you can write `docker run -ti --name test busybox sh`.

### Boolean

Boolean options look like `-d=false`. The value you
see is the default value which gets set if you do **not** use the
boolean flag. If you do call `run -d`, that sets the
opposite boolean value, so in this case, `true`, and
so `docker run -d` **will** run in "detached" mode,
in the background. Other boolean options are similar – specifying them
will set the value to the opposite of the default value.

### Multi

Options like `-a=[]` indicate they can be specified multiple times:

    $ docker run -a stdin -a stdout -a stderr -i -t ubuntu /bin/bash

Sometimes this can use a more complex value string, as for `-v`:

    $ docker run -v /host:/container example/mysql

### Strings and Integers

Options like `--name=""` expect a string, and they
can only be specified once. Options like `-c=0`
expect an integer, and they can only be specified once.

## daemon

    Usage of docker:
      --api-enable-cors=false                    Enable CORS headers in the remote API
      -b, --bridge=""                            Attach containers to a pre-existing network bridge
                                                   use 'none' to disable container networking
      --bip=""                                   Use this CIDR notation address for the network bridge's IP, not compatible with -b
      -d, --daemon=false                         Enable daemon mode
      -D, --debug=false                          Enable debug mode
      --dns=[]                                   Force docker to use specific DNS servers
      --dns-search=[]                            Force Docker to use specific DNS search domains
      -e, --exec-driver="native"                 Force the docker runtime to use a specific exec driver
      -G, --group="docker"                       Group to assign the unix socket specified by -H when running in daemon mode
                                                   use '' (the empty string) to disable setting of a group
      -g, --graph="/var/lib/docker"              Path to use as the root of the docker runtime
      -H, --host=[]                              The socket(s) to bind to in daemon mode
                                                   specified using one or more tcp://host:port, unix:///path/to/socket, fd://* or fd://socketfd.
      --icc=true                                 Enable inter-container communication
      --ip="0.0.0.0"                             Default IP address to use when binding container ports
      --ip-forward=true                          Enable net.ipv4.ip_forward
      --iptables=true                            Enable Docker's addition of iptables rules
      --mtu=0                                    Set the containers network MTU
                                                   if no value is provided: default to the default route MTU or 1500 if no default route is available
      -p, --pidfile="/var/run/docker.pid"        Path to use for daemon PID file
      -r, --restart=true                         Restart previously running containers
      -s, --storage-driver=""                    Force the docker runtime to use a specific storage driver
      --storage-opt=[]                           Set storage driver options
      --selinux-enabled=false                    Enable selinux support
      --tls=false                                Use TLS; implied by tls-verify flags
      --tlscacert="/home/sven/.docker/ca.pem"    Trust only remotes providing a certificate signed by the CA given here
      --tlscert="/home/sven/.docker/cert.pem"    Path to TLS certificate file
      --tlskey="/home/sven/.docker/key.pem"      Path to TLS key file
      --tlsverify=false                          Use TLS and verify the remote (daemon: verify client, client: verify daemon)
      -v, --version=false                        Print version information and quit

Options with [] may be specified multiple times.

The Docker daemon is the persistent process that manages containers.
Docker uses the same binary for both the daemon and client. To run the
daemon you provide the `-d` flag.

To force Docker to use devicemapper as the storage driver, use
`docker -d -s devicemapper`.

To set the DNS server for all Docker containers, use
`docker -d --dns 8.8.8.8`.

To set the DNS search domain for all Docker containers, use
`docker -d --dns-search example.com`.

To run the daemon with debug output, use `docker -d -D`.

To use lxc as the execution driver, use `docker -d -e lxc`.

The docker client will also honor the `DOCKER_HOST` environment variable to set
the `-H` flag for the client.

    $ docker -H tcp://0.0.0.0:2375 ps
    # or
    $ export DOCKER_HOST="tcp://0.0.0.0:2375"
    $ docker ps
    # both are equal

To run the daemon with [systemd socket activation](
http://0pointer.de/blog/projects/socket-activation.html), use
`docker -d -H fd://`. Using `fd://` will work perfectly for most setups but
you can also specify individual sockets too `docker -d -H fd://3`. If the
specified socket activated files aren't found then docker will exit. You
can find examples of using systemd socket activation with docker and
systemd in the [docker source tree](
https://github.com/dotcloud/docker/blob/master/contrib/init/systemd/socket-activation/).

Docker supports softlinks for the Docker data directory
(`/var/lib/docker`) and for `/tmp`. TMPDIR and the data directory can be set
like this:

    TMPDIR=/mnt/disk2/tmp /usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// > /var/lib/boot2docker/docker.log 2>&1
    # or
    export TMPDIR=/mnt/disk2/tmp
    /usr/local/bin/docker -d -D -g /var/lib/docker -H unix:// > /var/lib/boot2docker/docker.log 2>&1

## attach

    Usage: docker attach [OPTIONS] CONTAINER

    Attach to a running container

      --no-stdin=false    Do not attach stdin
      --sig-proxy=true    Proxify all received signal to the process (even in non-tty mode)

The `attach` command will allow you to view or
interact with any running container, detached (`-d`)
or interactive (`-i`). You can attach to the same
container at the same time - screen sharing style, or quickly view the
progress of your daemonized process.

You can detach from the container again (and leave it running) with
`CTRL-C` (for a quiet exit) or `CTRL-\`
to get a stacktrace of the Docker client when it quits. When
you detach from the container's process the exit code will be returned
to the client.

To stop a container, use `docker stop`.

To kill the container, use `docker kill`.

### Examples:

    $ ID=$(sudo docker run -d ubuntu /usr/bin/top -b)
    $ sudo docker attach $ID
    top - 02:05:52 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
    Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
    Cpu(s):  0.1%us,  0.2%sy,  0.0%ni, 99.7%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
    Mem:    373572k total,   355560k used,    18012k free,    27872k buffers
    Swap:   786428k total,        0k used,   786428k free,   221740k cached

    PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
     1 root      20   0 17200 1116  912 R    0  0.3   0:00.03 top

     top - 02:05:55 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.0%us,  0.2%sy,  0.0%ni, 99.8%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355244k used,    18328k free,    27872k buffers
     Swap:   786428k total,        0k used,   786428k free,   221776k cached

       PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
           1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top


     top - 02:05:58 up  3:06,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.2%us,  0.3%sy,  0.0%ni, 99.5%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355780k used,    17792k free,    27880k buffers
     Swap:   786428k total,        0k used,   786428k free,   221776k cached

     PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND
          1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top
    ^C$
    $ sudo docker stop $ID

## build

    Usage: docker build [OPTIONS] PATH | URL | -

    Build a new image from the source code at PATH

      --force-rm=false     Always remove intermediate containers, even after unsuccessful builds
      --no-cache=false     Do not use cache when building the image
      -q, --quiet=false    Suppress the verbose output generated by the containers
      --rm=true            Remove intermediate containers after a successful build
      -t, --tag=""         Repository name (and optionally a tag) to be applied to the resulting image in case of success

Use this command to build Docker images from a Dockerfile
and a "context".

The files at `PATH` or `URL` are called the "context" of the build. The build
process may refer to any of the files in the context, for example when using an
[*ADD*](/reference/builder/#dockerfile-add) instruction. When a single Dockerfile is
given as `URL` or is piped through STDIN (`docker build - < Dockerfile`), then
no context is set.

When a Git repository is set as `URL`, then the
repository is used as the context. The Git repository is cloned with its
submodules (git clone –recursive). A fresh git clone occurs in a
temporary directory on your local host, and then this is sent to the
Docker daemon as the context. This way, your local user credentials and
vpn's etc can be used to access private repositories.

See also:

[*Dockerfile Reference*](/reference/builder/#dockerbuilder).

### Examples:

    $ sudo docker build .
    Uploading context 10240 bytes
    Step 1 : FROM busybox
    Pulling repository busybox
     ---> e9aa60c60128MB/2.284 MB (100%) endpoint: https://cdn-registry-1.docker.io/v1/
    Step 2 : RUN ls -lh /
     ---> Running in 9c9e81692ae9
    total 24
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 bin
    drwxr-xr-x    5 root     root        4.0K Oct 19 00:19 dev
    drwxr-xr-x    2 root     root        4.0K Oct 19 00:19 etc
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 lib
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 lib64 -> lib
    dr-xr-xr-x  116 root     root           0 Nov 15 23:34 proc
    lrwxrwxrwx    1 root     root           3 Mar 12  2013 sbin -> bin
    dr-xr-xr-x   13 root     root           0 Nov 15 23:34 sys
    drwxr-xr-x    2 root     root        4.0K Mar 12  2013 tmp
    drwxr-xr-x    2 root     root        4.0K Nov 15 23:34 usr
     ---> b35f4035db3f
    Step 3 : CMD echo Hello World
     ---> Running in 02071fceb21b
     ---> f52f38b7823e
    Successfully built f52f38b7823e
    Removing intermediate container 9c9e81692ae9
    Removing intermediate container 02071fceb21b

This example specifies that the `PATH` is
`.`, and so all the files in the local directory get
`tar`d and sent to the Docker daemon. The `PATH`
specifies where to find the files for the "context" of the build on the
Docker daemon. Remember that the daemon could be running on a remote
machine and that no parsing of the Dockerfile
happens at the client side (where you're running
`docker build`). That means that *all* the files at
`PATH` get sent, not just the ones listed to
[*ADD*](/reference/builder/#dockerfile-add) in the Dockerfile.

The transfer of context from the local machine to the Docker daemon is
what the `docker` client means when you see the
"Sending build context" message.

If you wish to keep the intermediate containers after the build is
complete, you must use `--rm=false`. This does not
affect the build cache.

    $ sudo docker build -t vieux/apache:2.0 .

This will build like the previous example, but it will then tag the
resulting image. The repository name will be `vieux/apache`
and the tag will be `2.0`

    $ sudo docker build - < Dockerfile

This will read a Dockerfile from *stdin* without
context. Due to the lack of a context, no contents of any local
directory will be sent to the `docker` daemon. Since
there is no context, a Dockerfile `ADD`
only works if it refers to a remote URL.

    $ sudo docker build github.com/creack/docker-firefox

This will clone the GitHub repository and use the cloned repository as
context. The Dockerfile at the root of the
repository is used as Dockerfile. Note that you
can specify an arbitrary Git repository by using the `git://`
schema.

> **Note:** `docker build` will return a `no such file or directory` error
> if the file or directory does not exist in the uploaded context. This may
> happen if there is no context, or if you specify a file that is elsewhere 
> on the Host system. The context is limited to the current directory (and its
> children) for security reasons, and to ensure repeatable builds on remote
> Docker hosts. This is also the reason why `ADD ../file` will not work.

## commit

    Usage: docker commit [OPTIONS] CONTAINER [REPOSITORY[:TAG]]

    Create a new image from a container's changes

      -a, --author=""     Author (eg. "John Hannibal Smith <hannibal@a-team.com>"
      -m, --message=""    Commit message

It can be useful to commit a container's file changes or settings into a
new image. This allows you debug a container by running an interactive
shell, or to export a working dataset to another server. Generally, it
is better to use Dockerfiles to manage your images in a documented and
maintainable way.

### Commit an existing container

    $ sudo docker ps
    ID                  IMAGE               COMMAND             CREATED             STATUS              PORTS
    c3f279d17e0a        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    197387f1b436        ubuntu:12.04        /bin/bash           7 days ago          Up 25 hours
    $ docker commit c3f279d17e0a  SvenDowideit/testimage:version3
    f5283438590d
    $ docker images | head
    REPOSITORY                        TAG                 ID                  CREATED             VIRTUAL SIZE
    SvenDowideit/testimage            version3            f5283438590d        16 seconds ago      335.7 MB

## cp

Copy files/folders from the containers filesystem to the host
path.  Paths are relative to the root of the filesystem.

    Usage: docker cp CONTAINER:PATH HOSTPATH

    Copy files/folders from the PATH to the HOSTPATH

## diff

List the changed files and directories in a container᾿s filesystem

    Usage: docker diff CONTAINER

    Inspect changes on a container's filesystem

There are 3 events that are listed in the `diff`:

1.  `A` - Add
2.  `D` - Delete
3.  `C` - Change

For example:

    $ sudo docker diff 7bb0e258aefe

    C /dev
    A /dev/kmsg
    C /etc
    A /etc/mtab
    A /go
    A /go/src
    A /go/src/github.com
    A /go/src/github.com/dotcloud
    A /go/src/github.com/dotcloud/docker
    A /go/src/github.com/dotcloud/docker/.git
    ....

## events

    Usage: docker events [OPTIONS]

    Get real time events from the server

      --since=""         Show all events created since timestamp
      --until=""         Stream events until this timestamp

### Examples

You'll need two shells for this example.

**Shell 1: Listening for events:**

    $ sudo docker events

**Shell 2: Start and Stop a Container:**

    $ sudo docker start 4386fb97867d
    $ sudo docker stop 4386fb97867d

**Shell 1: (Again .. now showing events):**

    [2013-09-03 15:49:26 +0200 CEST] 4386fb97867d: (from 12de384bfb10) start
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

**Show events in the past from a specified time:**

    $ sudo docker events --since 1378216169
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

    $ sudo docker events --since '2013-09-03'
    [2013-09-03 15:49:26 +0200 CEST] 4386fb97867d: (from 12de384bfb10) start
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

    $ sudo docker events --since '2013-09-03 15:49:29 +0200 CEST'
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) die
    [2013-09-03 15:49:29 +0200 CEST] 4386fb97867d: (from 12de384bfb10) stop

## export

    Usage: docker export CONTAINER

    Export the contents of a filesystem as a tar archive to STDOUT

For example:

    $ sudo docker export red_panda > latest.tar

## history

    Usage: docker history [OPTIONS] IMAGE

    Show the history of an image

      --no-trunc=false     Don't truncate output
      -q, --quiet=false    Only show numeric IDs

To see how the `docker:latest` image was built:

    $ docker history docker
    IMAGE                                                              CREATED             CREATED BY                                                                                                                                                 SIZE
    3e23a5875458790b7a806f95f7ec0d0b2a5c1659bfc899c89f939f6d5b8f7094   8 days ago          /bin/sh -c #(nop) ENV LC_ALL=C.UTF-8                                                                                                                       0 B
    8578938dd17054dce7993d21de79e96a037400e8d28e15e7290fea4f65128a36   8 days ago          /bin/sh -c dpkg-reconfigure locales &&    locale-gen C.UTF-8 &&    /usr/sbin/update-locale LANG=C.UTF-8                                                    1.245 MB
    be51b77efb42f67a5e96437b3e102f81e0a1399038f77bf28cea0ed23a65cf60   8 days ago          /bin/sh -c apt-get update && apt-get install -y    git    libxml2-dev    python    build-essential    make    gcc    python-dev    locales    python-pip   338.3 MB
    4b137612be55ca69776c7f30c2d2dd0aa2e7d72059820abf3e25b629f887a084   6 weeks ago         /bin/sh -c #(nop) ADD jessie.tar.xz in /                                                                                                                   121 MB
    750d58736b4b6cc0f9a9abe8f258cef269e3e9dceced1146503522be9f985ada   6 weeks ago         /bin/sh -c #(nop) MAINTAINER Tianon Gravi <admwiggin@gmail.com> - mkimage-debootstrap.sh -t jessie.tar.xz jessie http://http.debian.net/debian             0 B
    511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158   9 months ago                                                                                                                                                                   0 B

## images

    Usage: docker images [OPTIONS] [NAME]

    List images

      -a, --all=false      Show all images (by default filter out the intermediate image layers)
      -f, --filter=[]:     Provide filter values (i.e. 'dangling=true')
      --no-trunc=false     Don't truncate output
      -q, --quiet=false    Only show numeric IDs

The default `docker images` will show all top level
images, their repository and tags, and their virtual size.

Docker images have intermediate layers that increase reuseability,
decrease disk usage, and speed up `docker build` by
allowing each step to be cached. These intermediate layers are not shown
by default.

### Listing the most recently created images

    $ sudo docker images | head
    REPOSITORY                    TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    <none>                        <none>              77af4d6b9913        19 hours ago        1.089 GB
    committest                    latest              b6fa739cedf5        19 hours ago        1.089 GB
    <none>                        <none>              78a85c484f71        19 hours ago        1.089 GB
    $ docker                        latest              30557a29d5ab        20 hours ago        1.089 GB
    <none>                        <none>              0124422dd9f9        20 hours ago        1.089 GB
    <none>                        <none>              18ad6fad3402        22 hours ago        1.082 GB
    <none>                        <none>              f9f1e26352f0        23 hours ago        1.089 GB
    tryout                        latest              2629d1fa0b81        23 hours ago        131.5 MB
    <none>                        <none>              5ed6274db6ce        24 hours ago        1.089 GB

### Listing the full length image IDs

    $ sudo docker images --no-trunc | head
    REPOSITORY                    TAG                 IMAGE ID                                                           CREATED             VIRTUAL SIZE
    <none>                        <none>              77af4d6b9913e693e8d0b4b294fa62ade6054e6b2f1ffb617ac955dd63fb0182   19 hours ago        1.089 GB
    committest                    latest              b6fa739cedf5ea12a620a439402b6004d057da800f91c7524b5086a5e4749c9f   19 hours ago        1.089 GB
    <none>                        <none>              78a85c484f71509adeaace20e72e941f6bdd2b25b4c75da8693efd9f61a37921   19 hours ago        1.089 GB
    $ docker                        latest              30557a29d5abc51e5f1d5b472e79b7e296f595abcf19fe6b9199dbbc809c6ff4   20 hours ago        1.089 GB
    <none>                        <none>              0124422dd9f9cf7ef15c0617cda3931ee68346455441d66ab8bdc5b05e9fdce5   20 hours ago        1.089 GB
    <none>                        <none>              18ad6fad340262ac2a636efd98a6d1f0ea775ae3d45240d3418466495a19a81b   22 hours ago        1.082 GB
    <none>                        <none>              f9f1e26352f0a3ba6a0ff68167559f64f3e21ff7ada60366e2d44a04befd1d3a   23 hours ago        1.089 GB
    tryout                        latest              2629d1fa0b81b222fca63371ca16cbf6a0772d07759ff80e8d1369b926940074   23 hours ago        131.5 MB
    <none>                        <none>              5ed6274db6ceb2397844896966ea239290555e74ef307030ebb01ff91b1914df   24 hours ago        1.089 GB

### Filtering

The filtering flag (-f or --filter) format is of "key=value". If there are more
than one filter, then pass multiple flags (e.g. `--filter "foo=bar" --filter "bif=baz"`)

Current filters:
 * dangling (boolean - true or false)

#### untagged images

    $ sudo docker images --filter "dangling=true"

    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    <none>              <none>              8abc22fbb042        4 weeks ago         0 B
    <none>              <none>              48e5f45168b9        4 weeks ago         2.489 MB
    <none>              <none>              bf747efa0e2f        4 weeks ago         0 B
    <none>              <none>              980fe10e5736        12 weeks ago        101.4 MB
    <none>              <none>              dea752e4e117        12 weeks ago        101.4 MB
    <none>              <none>              511136ea3c5a        8 months ago        0 B

This will display untagged images, that are the leaves of the images tree (not
intermediary layers). These images occur when a new build of an image takes the
repo:tag away from the IMAGE ID, leaving it untagged. A warning will be issued
if trying to remove an image when a container is presently using it.
By having this flag it allows for batch cleanup.

Ready for use by `docker rmi ...`, like:

    $ sudo docker rmi $(sudo docker images -f "dangling=true" -q)

    8abc22fbb042
    48e5f45168b9
    bf747efa0e2f
    980fe10e5736
    dea752e4e117
    511136ea3c5a

NOTE: Docker will warn you if any containers exist that are using these untagged images.


## import

    Usage: docker import URL|- [REPOSITORY[:TAG]]

    Create an empty filesystem image and import the contents of the tarball (.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz) into it, then optionally tag it.

URLs must start with `http` and point to a single
file archive (.tar, .tar.gz, .tgz, .bzip, .tar.xz, or .txz) containing a
root filesystem. If you would like to import from a local directory or
archive, you can use the `-` parameter to take the
data from *stdin*.

### Examples

**Import from a remote location:**

This will create a new untagged image.

    $ sudo docker import http://example.com/exampleimage.tgz

**Import from a local file:**

Import to docker via pipe and *stdin*.

    $ cat exampleimage.tgz | sudo docker import - exampleimagelocal:new

**Import from a local directory:**

    $ sudo tar -c . | sudo docker import - exampleimagedir

Note the `sudo` in this example – you must preserve
the ownership of the files (especially root ownership) during the
archiving with tar. If you are not root (or the sudo command) when you
tar, then the ownerships might not get preserved.

## info

    Usage: docker info

    Display system-wide information

For example:

    $ sudo docker info
    Containers: 292
    Images: 194
    Debug mode (server): false
    Debug mode (client): false
    Fds: 22
    Goroutines: 67
    LXC Version: 0.9.0
    EventsListeners: 115
    Kernel Version: 3.8.0-33-generic
    WARNING: No swap limit support

When sending issue reports, please use `docker version` and `docker info` to
ensure we know how your setup is configured.

## inspect

    Usage: docker inspect CONTAINER|IMAGE [CONTAINER|IMAGE...]

    Return low-level information on a container/image

      -f, --format=""    Format the output using the given go template.

By default, this will render all results in a JSON array. If a format is
specified, the given template will be executed for each result.

Go's [text/template](http://golang.org/pkg/text/template/) package
describes all the details of the format.

### Examples

**Get an instance'sIP Address:**

For the most part, you can pick out any field from the JSON in a fairly
straightforward manner.

    $ sudo docker inspect --format='{{.NetworkSettings.IPAddress}}' $INSTANCE_ID

**List All Port Bindings:**

One can loop over arrays and maps in the results to produce simple text
output:

    $ sudo docker inspect --format='{{range $p, $conf := .NetworkSettings.Ports}} {{$p}} -> {{(index $conf 0).HostPort}} {{end}}' $INSTANCE_ID

**Find a Specific Port Mapping:**

The `.Field` syntax doesn't work when the field name
begins with a number, but the template language's `index`
function does. The `.NetworkSettings.Ports`
section contains a map of the internal port mappings to a list
of external address/port objects, so to grab just the numeric public
port, you use `index` to find the specific port map,
and then `index` 0 contains first object inside of
that. Then we ask for the `HostPort` field to get
the public address.

    $ sudo docker inspect --format='{{(index (index .NetworkSettings.Ports "8787/tcp") 0).HostPort}}' $INSTANCE_ID

**Get config:**

The `.Field` syntax doesn't work when the field
contains JSON data, but the template language's custom `json`
function does. The `.config` section
contains complex json object, so to grab it as JSON, you use
`json` to convert config object into JSON

    $ sudo docker inspect --format='{{json .config}}' $INSTANCE_ID

## kill

    Usage: docker kill [OPTIONS] CONTAINER [CONTAINER...]

    Kill a running container (send SIGKILL, or specified signal)

      -s, --signal="KILL"    Signal to send to the container

The main process inside the container will be sent SIGKILL, or any
signal specified with option `--signal`.

## load

    Usage: docker load

    Load an image from a tar archive on STDIN

      -i, --input=""     Read from a tar archive file, instead of STDIN

Loads a tarred repository from a file or the standard input stream.
Restores both images and tags.

    $ sudo docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    $ sudo docker load < busybox.tar
    $ sudo docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    $ sudo docker load --input fedora.tar
    $ sudo docker images
    REPOSITORY          TAG                 IMAGE ID            CREATED             VIRTUAL SIZE
    busybox             latest              769b9341d937        7 weeks ago         2.489 MB
    fedora              rawhide             0d20aec6529d        7 weeks ago         387 MB
    fedora              20                  58394af37342        7 weeks ago         385.5 MB
    fedora              heisenbug           58394af37342        7 weeks ago         385.5 MB
    fedora              latest              58394af37342        7 weeks ago         385.5 MB

## login

    Usage: docker login [OPTIONS] [SERVER]

    Register or Login to a docker registry server, if no server is specified "https://index.docker.io/v1/" is the default.

      -e, --email=""       Email
      -p, --password=""    Password
      -u, --username=""    Username

If you want to login to a self-hosted registry you can
specify this by adding the server name.

    example:
    $ docker login localhost:8080

## logs

    Usage: docker logs CONTAINER

    Fetch the logs of a container

      -f, --follow=false        Follow log output
      -t, --timestamps=false    Show timestamps

The `docker logs` command batch-retrieves all logs
present at the time of execution.

The ``docker logs --follow`` command will first return all logs from the
beginning and then continue streaming new output from the container's stdout
and stderr.

## port

    Usage: docker port CONTAINER PRIVATE_PORT

    Lookup the public-facing port which is NAT-ed to PRIVATE_PORT

## ps

    Usage: docker ps [OPTIONS]

    List containers

      -a, --all=false       Show all containers. Only running containers are shown by default.
      --before=""           Show only container created before Id or Name, include non-running ones.
      -l, --latest=false    Show only the latest created container, include non-running ones.
      -n=-1                 Show n last created containers, include non-running ones.
      --no-trunc=false      Don't truncate output
      -q, --quiet=false     Only display numeric IDs
      -s, --size=false      Display sizes
      --since=""            Show only containers created since Id or Name, include non-running ones.

Running `docker ps` showing 2 linked containers.

    $ docker ps
    CONTAINER ID        IMAGE                        COMMAND                CREATED              STATUS              PORTS               NAMES
    4c01db0b339c        ubuntu:12.04                 bash                   17 seconds ago       Up 16 seconds                           webapp
    d7886598dbe2        crosbymichael/redis:latest   /redis-server --dir    33 minutes ago       Up 33 minutes       6379/tcp            redis,webapp/db

`docker ps` will show only running containers by default. To see all containers:
`docker ps -a`

## pull

    Usage: docker pull NAME[:TAG]

    Pull an image or a repository from the registry

Most of your images will be created on top of a base image from the
[Docker.io](https://index.docker.io) registry.

[Docker.io](https://index.docker.io) contains many pre-built images that you
can `pull` and try without needing to define and configure your own.

To download a particular image, or set of images (i.e., a repository),
use `docker pull`:

    $ docker pull debian
    # will pull all the images in the debian repository
    $ docker pull debian:testing
    # will pull only the image named debian:testing and any intermediate layers
    # it is based on. (typically the empty `scratch` image, a MAINTAINERs layer,
    # and the un-tared base.

## push

    Usage: docker push NAME[:TAG]

    Push an image or a repository to the registry

Use `docker push` to share your images to the [Docker.io](https://index.docker.io)
registry or to a self-hosted one.

## restart

    Usage: docker restart [OPTIONS] CONTAINER [CONTAINER...]

    Restart a running container

      -t, --time=10      Number of seconds to try to stop for before killing the container. Once killed it will then be restarted. Default=10

## rm

    Usage: docker rm [OPTIONS] CONTAINER [CONTAINER...]

    Remove one or more containers

      -f, --force=false      Force removal of running container
      -l, --link=false       Remove the specified link and not the underlying container
      -v, --volumes=false    Remove the volumes associated to the container

### Known Issues (rm)

-   [Issue 197](https://github.com/dotcloud/docker/issues/197) indicates
    that `docker kill` may leave directories behind
    and make it difficult to remove the container.

### Examples:

    $ sudo docker rm /redis
    /redis

This will remove the container referenced under the link
`/redis`.

    $ sudo docker rm --link /webapp/redis
    /webapp/redis

This will remove the underlying link between `/webapp`
and the `/redis` containers removing all
network communication.

    $ sudo docker rm $(docker ps -a -q)

This command will delete all stopped containers. The command
`docker ps -a -q` will return all existing container
IDs and pass them to the `rm` command which will
delete them. Any running containers will not be deleted.

## rmi

    Usage: docker rmi IMAGE [IMAGE...]

    Remove one or more images

      -f, --force=false    Force
      --no-prune=false     Do not delete untagged parents

### Removing tagged images

Images can be removed either by their short or long ID`s, or their image
names. If an image has more than one name, each of them needs to be
removed before the image is removed.

    $ sudo docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             SIZE
    test1                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test                      latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    test2                     latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)

    $ sudo docker rmi fd484f19954f
    Error: Conflict, cannot delete image fd484f19954f because it is tagged in multiple repositories
    2013/12/11 05:47:16 Error: failed to remove one or more images

    $ sudo docker rmi test1
    Untagged: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8
    $ sudo docker rmi test2
    Untagged: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8

    $ sudo docker images
    REPOSITORY                TAG                 IMAGE ID            CREATED             SIZE
    test                      latest              fd484f19954f        23 seconds ago      7 B (virtual 4.964 MB)
    $ sudo docker rmi test
    Untagged: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8
    Deleted: fd484f19954f4920da7ff372b5067f5b7ddb2fd3830cecd17b96ea9e286ba5b8

## run

    Usage: docker run [OPTIONS] IMAGE [COMMAND] [ARG...]

    Run a command in a new container

      -a, --attach=[]            Attach to stdin, stdout or stderr.
      -c, --cpu-shares=0         CPU shares (relative weight)
      --cidfile=""               Write the container ID to the file
      -d, --detach=false         Detached mode: Run container in the background, print new container id
      --dns=[]                   Set custom dns servers
      --dns-search=[]            Set custom dns search domains
      -e, --env=[]               Set environment variables
      --entrypoint=""            Overwrite the default entrypoint of the image
      --env-file=[]              Read in a line delimited file of ENV variables
      --expose=[]                Expose a port from the container without publishing it to your host
      -h, --hostname=""          Container host name
      -i, --interactive=false    Keep stdin open even if not attached
      --link=[]                  Add link to another container (name:alias)
      --lxc-conf=[]              (lxc exec-driver only) Add custom lxc options --lxc-conf="lxc.cgroup.cpuset.cpus = 0,1"
      -m, --memory=""            Memory limit (format: <number><optional unit>, where unit = b, k, m or g)
      --name=""                  Assign a name to the container
      --net="bridge"             Set the Network mode for the container
                                   'bridge': creates a new network stack for the container on the docker bridge
                                   'none': no networking for this container
                                   'container:<name|id>': reuses another container network stack
                                   'host': use the host network stack inside the contaner
      -p, --publish=[]           Publish a container's port to the host
                                   format: ip:hostPort:containerPort | ip::containerPort | hostPort:containerPort
                                   (use 'docker port' to see the actual mapping)
      -P, --publish-all=false    Publish all exposed ports to the host interfaces
      --privileged=false         Give extended privileges to this container
      --rm=false                 Automatically remove the container when it exits (incompatible with -d)
      --sig-proxy=true           Proxify all received signal to the process (even in non-tty mode)
      -t, --tty=false            Allocate a pseudo-tty
      -u, --user=""              Username or UID
      -v, --volume=[]            Bind mount a volume (e.g. from the host: -v /host:/container, from docker: -v /container)
      --volumes-from=[]          Mount volumes from the specified container(s)
      -w, --workdir=""           Working directory inside the container

The `docker run` command first `creates` a writeable container layer over the
specified image, and then `starts` it using the specified command. That is,
`docker run` is equivalent to the API `/containers/create` then
`/containers/(id)/start`. A stopped container can be restarted with all its
previous changes intact using `docker start`. See `docker ps -a` to view a list
of all containers.

The `docker run` command can be used in combination with `docker commit` to
[*change the command that a container runs*](#commit-an-existing-container).

See the [Docker User Guide](/userguide/dockerlinks/) for more detailed
information about the `--expose`, `-p`, `-P` and `--link` parameters,
and linking containers.

### Known Issues (run –volumes-from)

- [Issue 2702](https://github.com/dotcloud/docker/issues/2702):
  "lxc-start: Permission denied - failed to mount" could indicate a
  permissions problem with AppArmor. Please see the issue for a
  workaround.

### Examples:

    $ sudo docker run --cidfile /tmp/docker_test.cid ubuntu echo "test"

This will create a container and print `test` to the console. The `cidfile`
flag makes Docker attempt to create a new file and write the container ID to it.
If the file exists already, Docker will return an error. Docker will close this
file when `docker run` exits.

    $ sudo docker run -t -i --rm ubuntu bash
    root@bc338942ef20:/# mount -t tmpfs none /mnt
    mount: permission denied

This will *not* work, because by default, most potentially dangerous kernel
capabilities are dropped; including `cap_sys_admin` (which is required to mount
filesystems). However, the `--privileged` flag will allow it to run:

    $ sudo docker run --privileged ubuntu bash
    root@50e3f57e16e6:/# mount -t tmpfs none /mnt
    root@50e3f57e16e6:/# df -h
    Filesystem      Size  Used Avail Use% Mounted on
    none            1.9G     0  1.9G   0% /mnt

The `--privileged` flag gives *all* capabilities to the container, and it also
lifts all the limitations enforced by the `device` cgroup controller. In other
words, the container can then do almost everything that the host can do. This
flag exists to allow special use-cases, like running Docker within Docker.

    $ sudo docker  run -w /path/to/dir/ -i -t  ubuntu pwd

The `-w` lets the command being executed inside directory given, here
`/path/to/dir/`. If the path does not exists it is created inside the container.

    $ sudo docker  run  -v `pwd`:`pwd` -w `pwd` -i -t  ubuntu pwd

The `-v` flag mounts the current working directory into the container. The `-w`
lets the command being executed inside the current working directory, by
changing into the directory to the value returned by `pwd`. So this
combination executes the command using the container, but inside the
current working directory.

    $ sudo docker run -v /doesnt/exist:/foo -w /foo -i -t ubuntu bash

When the host directory of a bind-mounted volume doesn't exist, Docker
will automatically create this directory on the host for you. In the
example above, Docker will create the `/doesnt/exist`
folder before starting your container.

    $ sudo docker run -t -i -v /var/run/docker.sock:/var/run/docker.sock -v ./static-docker:/usr/bin/docker busybox sh

By bind-mounting the docker unix socket and statically linked docker
binary (such as that provided by [https://get.docker.io](
https://get.docker.io)), you give the container the full access to create and
manipulate the host's docker daemon.

    $ sudo docker run -p 127.0.0.1:80:8080 ubuntu bash

This binds port `8080` of the container to port `80` on `127.0.0.1` of
the host machine. The [Docker User Guide](/userguide/dockerlinks/)
explains in detail how to manipulate ports in Docker.

    $ sudo docker run --expose 80 ubuntu bash

This exposes port `80` of the container for use within a link without
publishing the port to the host system's interfaces. The [Docker User
Guide](/userguide/dockerlinks) explains in detail how to manipulate
ports in Docker.

    $ sudo docker run -e MYVAR1 --env MYVAR2=foo --env-file ./env.list ubuntu bash

This sets environmental variables in the container. For illustration all three
flags are shown here. Where `-e`, `--env` take an environment variable and
value, or if no "=" is provided, then that variable's current value is passed
through (i.e. $MYVAR1 from the host is set to $MYVAR1 in the container). All
three flags, `-e`, `--env` and `--env-file` can be repeated.

Regardless of the order of these three flags, the `--env-file` are processed
first, and then `-e`, `--env` flags. This way, the `-e` or `--env` will
override variables as needed.

    $ cat ./env.list
    TEST_FOO=BAR
    $ sudo docker run --env TEST_FOO="This is a test" --env-file ./env.list busybox env | grep TEST_FOO
    TEST_FOO=This is a test

The `--env-file` flag takes a filename as an argument and expects each line
to be in the VAR=VAL format, mimicking the argument passed to `--env`. Comment
lines need only be prefixed with `#`

An example of a file passed with `--env-file`

    $ cat ./env.list
    TEST_FOO=BAR

    # this is a comment
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888

    # pass through this variable from the caller
    TEST_PASSTHROUGH
    $ sudo TEST_PASSTHROUGH=howdy docker run --env-file ./env.list busybox env
    HOME=/
    PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
    HOSTNAME=5198e0745561
    TEST_FOO=BAR
    TEST_APP_DEST_HOST=10.10.0.127
    TEST_APP_DEST_PORT=8888
    TEST_PASSTHROUGH=howdy

    $ sudo docker run --name console -t -i ubuntu bash

This will create and run a new container with the container name being
`console`.

    $ sudo docker run --link /redis:redis --name console ubuntu bash

The `--link` flag will link the container named `/redis` into the newly
created container with the alias `redis`. The new container can access the
network and environment of the redis container via environment variables.
The `--name` flag will assign the name `console` to the newly created
container.

    $ sudo docker run --volumes-from 777f7dc92da7,ba8c0c54f0f2:ro -i -t ubuntu pwd

The `--volumes-from` flag mounts all the defined volumes from the referenced
containers. Containers can be specified by a comma separated list or by
repetitions of the `--volumes-from` argument. The container ID may be
optionally suffixed with `:ro` or `:rw` to mount the volumes in read-only
or read-write mode, respectively. By default, the volumes are mounted in
the same mode (read write or read only) as the reference container.

The `-a` flag tells `docker run` to bind to the container's stdin, stdout or
stderr. This makes it possible to manipulate the output and input as needed.

    $ echo "test" | sudo docker run -i -a stdin ubuntu cat -

This pipes data into a container and prints the container's ID by attaching
only to the container'sstdin.

    $ sudo docker run -a stderr ubuntu echo test

This isn't going to print anything unless there's an error because We've
only attached to the stderr of the container. The container's logs still
   store what's been written to stderr and stdout.

    $ cat somefile | sudo docker run -i -a stdin mybuilder dobuild

This is how piping a file into a container could be done for a build.
The container's ID will be printed after the build is done and the build
logs could be retrieved using `docker logs`. This is
useful if you need to pipe a file or something else into a container and
retrieve the container's ID once the container has finished running.

**A complete example:**

    $ sudo docker run -d --name static static-web-files sh
    $ sudo docker run -d --expose=8098 --name riak riakserver
    $ sudo docker run -d -m 100m -e DEVELOPMENT=1 -e BRANCH=example-code -v $(pwd):/app/bin:ro --name app appserver
    $ sudo docker run -d -p 1443:443 --dns=dns.dev.org --dns-search=dev.org -v /var/log/httpd --volumes-from static --link riak --link app -h www.sven.dev.org --name web webserver
    $ sudo docker run -t -i --rm --volumes-from web -w /var/log/httpd busybox tail -f access.log

This example shows 5 containers that might be set up to test a web
application change:

1. Start a pre-prepared volume image `static-web-files` (in the background)
   that has CSS, image and static HTML in it, (with a `VOLUME` instruction in
   the Dockerfile to allow the web server to use those files);
2. Start a pre-prepared `riakserver` image, give the container name `riak` and
   expose port `8098` to any containers that link to it;
3. Start the `appserver` image, restricting its memory usage to 100MB, setting
   two environment variables `DEVELOPMENT` and `BRANCH` and bind-mounting the
   current directory (`$(pwd)`) in the container in read-only mode as `/app/bin`;
4. Start the `webserver`, mapping port `443` in the container to port `1443` on
   the Docker server, setting the DNS server to `dns.dev.org` and DNS search
   domain to `dev.org`, creating a volume to put the log files into (so we can
   access it from another container), then importing the files from the volume
   exposed by the `static` container, and linking to all exposed ports from
   `riak` and `app`. Lastly, we set the hostname to `web.sven.dev.org` so its
   consistent with the pre-generated SSL certificate;
5. Finally, we create a container that runs `tail -f access.log` using the logs
   volume from the `web` container, setting the workdir to `/var/log/httpd`. The
   `--rm` option means that when the container exits, the container's layer is
   removed.

## save

    Usage: docker save IMAGE

    Save an image to a tar archive (streamed to stdout by default)

      -o, --output=""    Write to an file, instead of STDOUT

Produces a tarred repository to the standard output stream. Contains all
parent layers, and all tags + versions, or specified repo:tag.

It is used to create a backup that can then be used with
`docker load`

    $ sudo docker save busybox > busybox.tar
    $ ls -sh busybox.tar
    2.7M busybox.tar
    $ sudo docker save --output busybox.tar busybox
    $ ls -sh busybox.tar
    2.7M busybox.tar
    $ sudo docker save -o fedora-all.tar fedora
    $ sudo docker save -o fedora-latest.tar fedora:latest

## search

Search [Docker.io](https://index.docker.io) for images

    Usage: docker search TERM

    Search the docker index for images

      --no-trunc=false       Don't truncate output
      -s, --stars=0          Only displays with at least xxx stars
      --automated=false      Only show automated builds

See [*Find Public Images on Docker.io*](
/userguide/dockerrepos/#find-public-images-on-dockerio) for
more details on finding shared images from the commandline.

## start

    Usage: docker start CONTAINER [CONTAINER...]

    Restart a stopped container

      -a, --attach=false         Attach container's stdout/stderr and forward all signals to the process
      -i, --interactive=false    Attach container's stdin

When run on a container that has already been started, 
takes no action and succeeds unconditionally.

## stop

    Usage: docker stop [OPTIONS] CONTAINER [CONTAINER...]

    Stop a running container (Send SIGTERM, and then SIGKILL after grace period)

      -t, --time=10      Number of seconds to wait for the container to stop before killing it.

The main process inside the container will receive SIGTERM, and after a
grace period, SIGKILL

## tag

    Usage: docker tag [OPTIONS] IMAGE [REGISTRYHOST/][USERNAME/]NAME[:TAG]

    Tag an image into a repository

      -f, --force=false    Force

You can group your images together using names and tags, and then upload
them to [*Share Images via Repositories*](
/userguide/dockerrepos/#working-with-the-repository).

## top

    Usage: docker top CONTAINER [ps OPTIONS]

    Lookup the running processes of a container

## version

    Usage: docker version

    Show the docker version information.

Show the Docker version, API version, Git commit, and Go version of
both Docker client and daemon.

## wait

    Usage: docker wait CONTAINER [CONTAINER...]

    Block until a container stops, then print its exit code.
