page_title: Kitematic User Guide: Intro & Overview
page_description: Documentation that provides an overview of Kitematic and installation instructions
page_keywords: docker, documentation, about, technology, kitematic, gui

# Kitematic user guide

## Overview

Kitematic is an open source project built to simplify and streamline using
Docker on a Mac or Windows (coming soon) PC. Kitematic automates the Docker
installation and setup process and provides an intuitive graphical user
interface (GUI) for running Docker containers.  Kitematic integrates with
[Docker Machine](http://docs.docker.com/machine/) to provision a VirtualBox VM
and install the Docker Engine locally on your machine.

Once installed, the Kitematic GUI launches and from the home screen you will be
presented with curated images that you can run instantly. You can search for any
public images on Docker Hub from Kitematic just by typing in the search bar. 
You can use the GUI to create, run and manage your containers just by clicking
on buttons. Kitematic allows you to switch back and forth between the Docker CLI
and the GUI.  Kitematic also automates advanced features such as managing ports
and configuring volumes.  You can use Kitematic to change environment variables,
stream logs, and single click terminal into your Docker container all from the
GUI.

First, if you haven't yet done so, [download and start
Kitematic](./index.md).

## Container list

Kitematic lists all running and stopped containers on the left side, underneath
the "New Container" link.

The container list includes all containers, even those not started by Kitematic,
giving you a quick over-view of the state of your Docker daemon.

You can click on any container to view its logs (the output of the main container
process), restart, stop or exec `sh` in that container. See [Working with a
container](#working-with-a-container) for more details.

## Creating a new container

The "New Container" page lets you search for and select from images on the Docker Hub.
When you've found the image you want to run, you can click "Create" to pull, create,
and run the container.

![Nginx create](../assets/browse-images.png)

## Working with a container

If you select a non-running container, either stopped, or paused, you will be able
to "Restart" or "Stop" the container using the icons. You can also view the entire
main container process' output logs, and in the Settings section you can make
changes which will be used if you "Restart" this container.

By selecting a running container from the left list, you can see some state information
for your container - either a preview of the HTML output for a container that has a web
server, the main container process' logs, and any container volumes that have been
configured.

![Redis container in Kitematic](../assets/cli-redis-container.png)

The summary page will show different things depending on the image metadata. If
a known "web" port (see below) is `EXPOSED`, then Kitematic assumes its a web page,
and will show a preview of the site at `/`. If other ports are exposed, then it
will show a list of those ports, and the Docker daemon IP and port they are mapped
to. If there are any `VOLUMES`, then these will be shown. At minimum, the summary
screen will show the main container process' log output.

The currently detected "web" ports are, `80`, `8000`, `8080`, `3000`, `5000`,
`2368`, `9200`, and `8983`.

### Viewing container logs

You can view the entire main container process' log output either by cicking on the "Logs"
preview image, or by clicking on the "Logs" tab.

You can then scroll through the logs from the current running container. Note that
if you make changes to the container Settings, then the container will be restarted,
so will reset this log view.

### Starting a terminal in a container

The "Terminal" icon at the top of the container summary will `docker exec sh <your container>`.
This will allow you to make quick changes, or to debug a problem. 

> **Note**: Your exec'ed `sh` process will not have the same environment settings
> as the main container process and its children.

### Managing Volumes

You can choose to make all of a container's volumes mapped to directories on
on your Mac by clicking on the folders in the "Edit Files" section of the
container summary screen.

This allows you to manage files in volumes via the Finder.
Kitematic exposes a container's volume data under `~/Kitematic/<container's name>/`.
Quick access to this folder (or directory) is available via the app:

![Accessing the volumes directory](../assets/volumes-dir.png)

> **Note**: When you "Enable all volumes to edit files in Finder", the Docker
> container will be stopped, removed and re-created with the new `volumes`
> flag.

#### Changing Volume Directories

Let's say you have an Nginx webserver running via Kitematic (using the
`kitematic/hello-world-nginx` image on DockerHub). However, you don't want to
use the default directory created for the website_files volume. Instead, you
already have the HTML, Javascript, and CSS for your website under
`~/workspace/website`.

Navigate to the "Settings" tab of the container, and goto the "Volumes". This
screen allows you to set the mappings individually.

![screen shot 2015-02-28 at 2 48 01 pm](../assets/change-folder.png)

> **Note**: When you "Change Folders", the Docker
> container will be stopped, removed and re-created with the new `volumes`
> flag.

### Setting the container name

By default, Kitematic sets the container name to the same as the image name (or
with a `-<number>` if there are more than one.
To simplify administration, or when using container linking or volumes, you may
want to rename it.

> **Note**: When you rename the container it will be stopped, removed and
> re-created with the new name (due to the default volumes mapping).

### Adding Environment variables

Many images use environment variables to let you customise them. The "General"
"Settings" tab allows you to add and modify the environment variables used to
start a container.

The list of Environment variables will show any that have been set on the image
metadata - for example, using the `ENV` instruction in the Dockerfile.

<TODO: image of the jenkins container>

Wen you "Save" the changed environment variables, the container will be stopped
removed and re-created.

### Delete container

On the "General" "Settings" tab, you can delete the container. Clicking "Delete
Container" will also stop the container if necessary.

You can also delete a container clicking the `X` icon in the container list.

Kitematic will prompt you to confirm that you want to delete.

#### List the exposed Ports and how to access them

To see the complete list of exposed ports, go to "Settings" then "Ports". This
page lists all the container ports exposed, and the IP address and host only
network port that you can access use to access that container from your OS X
system.

## Docker Command-line Access

You can interact with existing containers in Kitematic or create new containers
via the Docker Command Line Interface (CLI). Any changes you make on the CLI are
directly reflected in Kitematic.

To open a terminal via Kitematic, just press whale button at the bottom left, as
shown below:

![CLI access button](../assets/cli-access-button.png)

### Example: Creating a new Redis container

Start by opening a Docker-CLI ready terminal by clicking the whale button as
described above. Once the terminal opens, enter `docker run -d -P redis`. This
will pull and run a new Redis container via the Docker CLI.

![Docker CLI terminal window](../assets/cli-terminal.png)

> **Note**: If you're creating containers from the commandline, use `docker run -d`
> so that Kitematic can re-create the container when settings are changed via the
> Kitematic user interface. containers started without `-d` will fail to re-start.

Now, go back to Kitematic. The Redis container should now be visible.

![Redis container in Kitematic](../assets/cli-redis-container.png)

## Next Steps

For an example using Kitematic to run a Minecraft server, take a look at
the [Mincraft server](./minecraft-server.md) page.
