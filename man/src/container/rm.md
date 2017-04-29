**docker container rm** will remove one or more containers from the host node. The
container name or ID can be used. This does not remove images. You cannot
remove a running container unless you use the **-f** option. To see all
containers on a host use the **docker container ls -a** command.

# EXAMPLES

## Removing a container using its ID

To remove a container using its ID, find either from a **docker ps -a**
command, or use the ID returned from the **docker run** command, or retrieve
it from a file used to store it using the **docker run --cidfile**:

    docker container rm abebf7571666

## Removing a container using the container name

The name of the container can be found using the **docker ps -a**
command. The use that name as follows:

    docker container rm hopeful_morse

## Removing a container and all associated volumes

    $ docker container rm -v redis
    redis

This command will remove the container and any volumes associated with it.
Note that if a volume was specified with a name, it will not be removed.

    $ docker create -v awesome:/foo -v /bar --name hello redis
    hello
    $ docker container rm -v hello

In this example, the volume for `/foo` will remain in tact, but the volume for
`/bar` will be removed. The same behavior holds for volumes inherited with
`--volumes-from`.
