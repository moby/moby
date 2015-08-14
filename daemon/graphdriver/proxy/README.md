## proxy - a storage backend transparently redirecting all requests to another backend

### Rationale

It would be nice to run docker inside a container, but a docker graphdriver
sometimes need access to critical system-wide resources. For example,
devicemapper need access to /dev/loop and /dev/mapper/control, and it wants
to format and mount ext4 filesystems over devmapper devices.

Also, the superuser of container may mistakenly or maliciously modify the
content of raw devices that inner filesystem and dm-thin target works on.
Such modifications may havoc the whole host system and cannot be easily
isolated as per-container errors.

### Solution

Run a docker daemon in proxy mode on host system (outside container). It
listens on a unix socket (accessible from inside container) for commands from
a docker daemon running inside container. These command set is exactly docker
graphdriver API wrapped in the go net/rpc.

Then run a docker daemon on top of proxy graphdriver inside container. The
proxy graphdriver gets the address of remote server (i.e. host docker daemon
in proxy mode) via "--storage-opt" options interface and connects to the server
on startup. Then it transparently passes all incoming requests (in the form of
graphdriver API) to the docker daemon in proxy mode on host system.

The latter processes incoming Init request intelligently while passing all
other requests transparently to the actual graphdriver.

Such an approach factors out potentially dangerous operations to the trusted
host environment (assuming that container superuser cannot modify binaries on
host system) and also allow to keep per-container docker graphdriver files in
a protected area dedicated to the container.

### Step-by-step example

The example assumes that the docker daemon is already running on the host
system and a docker development container was started by:

``docker run --privileged --rm --name devcon -ti -v /root/repos/docker-fork:/go/src/github.com/docker/docker devcon-image /bin/bash``

Then, on the host system:

``docker proxydaemon -R /protected -C devcon -S unix:///root/repos/docker-fork/sock``

The command ``proxydaemon`` means start daemon in proxy mode.

Here ``-R /protected`` specifies the prefix to be added to Init request. So, if
the container user requested `/var/lib/docker` as the home of docker files, the
host system will actually keep them in `/protected/var/lib/docker`.

``-C devcon`` specifies the name of container (`devcon`) that
given instance of proxy daemon works for.

``-S host=unix:///root/repos/docker-fork/sock`` specifies the path
to the unix socket for daemon-to-daemon communication. It must be visible
from inside container.

Secondly, inside container:

``docker daemon -s proxy --storage-opt graphdriver=devicemapper --storage-opt proxyserver=unix:///go/src/github.com/docker/docker/sock``

Here ``-s proxy`` forces using proxy graphdriver. ``--storage-opt`` passes next
argument as an option for graphdriver. ``graphdriver=devicemapper`` specifies
graphdriver to use on remote side (on the host system) and
``proxyserver=unix:///go/src/github.com/docker/docker/sock`` specifies the path
to communication unix socket. Of course, this must strictly match to the
unix-socket path set for host docker daemon in proxy mode.

Since now, a person can run all variety of ``docker run/pull/etc ...`` inside
`devcon` container. The communication path to the host graphdriver must work
transparently for the person. In the other words, the presence of the proxy server
must be invisible for docker client inside container.
