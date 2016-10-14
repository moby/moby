---
title: "Docker commands"
description: "Docker's CLI command description and usage"
keywords: ["Docker, Docker documentation, CLI,  command line"]
identifier: "smn_cli_guide"
---



# The Docker commands

This section contains reference information on using Docker's command line
client. Each command has a reference page along with samples. If you are
unfamiliar with the command line, you should start by reading about how to [Use
the Docker command line](cli.md).

You start the Docker daemon with the command line. How you start the daemon
affects your Docker containers. For that reason you should also make sure to
read the [`dockerd`](dockerd.md) reference page.

### Docker management commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [dockerd](dockerd.md) | Launch the Docker daemon                             |
| [info](info.md) | Display system-wide information                            |
| [inspect](inspect.md)| Return low-level information on a container or image  |
| [version](version.md) | Show the Docker version information                  |


### Image commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [build](build.md) |  Build an image from a Dockerfile                        |
| [commit](commit.md) | Create a new image from a container's changes          |
| [history](history.md) | Show the history of an image                         |
| [images](images.md) | List images                                            |
| [import](import.md) | Import the contents from a tarball to create a filesystem image |
| [load](load.md) | Load an image from a tar archive or STDIN                  |
| [rmi](rmi.md) | Remove one or more images                                    |
| [save](save.md) | Save images to a tar archive                               |
| [tag](tag.md) | Tag an image into a repository                               |

### Container commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [attach](attach.md) | Attach to a running container                          |
| [cp](cp.md) | Copy files/folders from a container to a HOSTDIR or to STDOUT  |
| [create](create.md) | Create a new container                                 |
| [diff](diff.md) | Inspect changes on a container's filesystem                |
| [events](events.md) | Get real time events from the server                   |
| [exec](exec.md) | Run a command in a running container                       |
| [export](export.md) | Export a container's filesystem as a tar archive       |
| [kill](kill.md) | Kill a running container                                   |
| [logs](logs.md) | Fetch the logs of a container                              |
| [pause](pause.md) | Pause all processes within a container                   |
| [port](port.md) | List port mappings or a specific mapping for the container |
| [ps](ps.md) | List containers                                                |
| [rename](rename.md) | Rename a container                                     |
| [restart](restart.md) | Restart a running container                          |
| [rm](rm.md) | Remove one or more containers                                  |
| [run](run.md) | Run a command in a new container                             |
| [start](start.md) | Start one or more stopped containers                     |
| [stats](stats.md) | Display a live stream of container(s) resource usage  statistics |
| [stop](stop.md) | Stop a running container                                   |
| [top](top.md) | Display the running processes of a container                 |
| [unpause](unpause.md) | Unpause all processes within a container             |
| [update](update.md) | Update configuration of one or more containers         |
| [wait](wait.md) | Block until a container stops, then print its exit code    |

### Hub and registry commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [login](login.md) | Register or log in to a Docker registry                  |
| [logout](logout.md) | Log out from a Docker registry                         |
| [pull](pull.md) | Pull an image or a repository from a Docker registry       |
| [push](push.md) | Push an image or a repository to a Docker registry         |
| [search](search.md) | Search the Docker Hub for images                       |

### Network and connectivity commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [network connect](network_connect.md) | Connect a container to a network     |
| [network create](network_create.md) | Create a new network                   |
| [network disconnect](network_disconnect.md) | Disconnect a container from a network |
| [network inspect](network_inspect.md) | Display information about a network  |
| [network ls](network_ls.md) | Lists all the networks the Engine `daemon` knows about |
| [network rm](network_rm.md) | Removes one or more networks                   |


### Shared data volume commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [volume create](volume_create.md) | Creates a new volume where containers can consume and store data |
| [volume inspect](volume_inspect.md) | Display information about a volume     |
| [volume ls](volume_ls.md) | Lists all the volumes Docker knows about         |
| [volume rm](volume_rm.md) | Remove one or more volumes                       |


### Swarm node commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [node promote](node_promote.md) | Promote a node that is pending a promotion to manager |
| [node demote](node_demote.md) | Demotes an existing manager so that it is no longer a manager |
| [node inspect](node_inspect.md) | Inspect a node in the swarm                |
| [node update](node_update.md) | Update attributes for a node                 |
| [node ps](node_ps.md) | List tasks running on one or more nodes                         |
| [node ls](node_ls.md) | List nodes in the swarm                              |
| [node rm](node_rm.md) | Remove one or more nodes from the swarm                         |

### Swarm swarm commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [swarm init](swarm_init.md) | Initialize a swarm                             |
| [swarm join](swarm_join.md) | Join a swarm as a manager node or worker node  |
| [swarm leave](swarm_leave.md) | Remove the current node from the swarm       |
| [swarm update](swarm_update.md) | Update attributes of a swarm               |
| [swarm join-token](swarm_join_token.md) | Display or rotate join tokens      |

### Swarm service commands

| Command | Description                                                        |
|:--------|:-------------------------------------------------------------------|
| [service create](service_create.md) | Create a new service                   |
| [service inspect](service_inspect.md) | Inspect a service                    |
| [service ls](service_ls.md) | List services in the swarm                     |
| [service rm](service_rm.md) | Remove a service from the swarm                |
| [service scale](service_scale.md) | Set the number of replicas for the desired state of the service |
| [service ps](service_ps.md) | List the tasks of a service              |
| [service update](service_update.md)  | Update the attributes of a service    |
