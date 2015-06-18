# Experimental: Networking and Services

In this feature:

- `network` become a first class objects in the Docker UI

This is an experimental feature. For information on installing and using experimental features, see [the experimental feature overview](experimental.md).

## Using Networks

        Usage: docker network [OPTIONS] COMMAND [OPTIONS] [arg...]

        Commands:
            create                   Create a network
            rm                       Remove a network
            ls                       List all networks
            info                     Display information of a network

        Run 'docker network COMMAND --help' for more information on a command.

          --help=false       Print usage

The `docker network` command is used to manage Networks.

To create a network, `docker network create foo`. You can also specify a driver
if you have loaded a networking plugin e.g `docker network create -d <plugin_name> foo`

        $ docker network create foo
        aae601f43744bc1f57c515a16c8c7c4989a2cad577978a32e6910b799a6bccf6
        $ docker network create -d overlay bar
        d9989793e2f5fe400a58ef77f706d03f668219688ee989ea68ea78b990fa2406

`docker network ls` is used to display the currently configured networks

        $ docker network ls
        NETWORK ID          NAME                TYPE
        d367e613ff7f        none                null
        bd61375b6993        host                host
        cc455abccfeb        bridge              bridge
        aae601f43744        foo                 bridge
        d9989793e2f5        bar                 overlay

To get detailed information on a network, you can use the `docker network info`
command.

        $ docker network info foo
        Network Id: aae601f43744bc1f57c515a16c8c7c4989a2cad577978a32e6910b799a6bccf6
        Name: foo
        Type: null

If you no longer have need of a network, you can delete it with `docker network rm`

        $ docker network rm bar
        bar
        $ docker network ls
        NETWORK ID          NAME                TYPE
        aae601f43744        foo                 bridge
        d367e613ff7f        none                null
        bd61375b6993        host                host
        cc455abccfeb        bridge              bridge


Currently the only way this network can be used to connect container is via default network-mode.
Docker daemon supports a configuration flag `--default-network` which takes configuration value of format `NETWORK:DRIVER`, where,
`NETWORK` is the name of the network created using the `docker network create` command and 
`DRIVER` represents the in-built drivers such as bridge, overlay, container, host and none. or Remote drivers via Network Plugins.
When a container is created and if the network mode (`--net`) is not specified, then this default network will be used to connect
the container. If `--default-network` is not specified, the default network will be the `bridge` driver.

Send us feedback and comments on [#](https://github.com/docker/docker/issues/?),
or on the usual Google Groups (docker-user, docker-dev) and IRC channels.

