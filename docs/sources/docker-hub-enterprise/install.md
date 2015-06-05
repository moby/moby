no_version_dropdown: true
page_title: Docker Hub Enterprise: Install
page_description: Installation instructions for Docker Hub Enterprise
page_keywords: docker, documentation, about, technology, understanding, enterprise, hub, registry

# Installing Docker Hub Enterprise

## Overview

This document describes the process of obtaining, installing, and securing
Docker Hub Enterprise (DHE). DHE is installed from Docker containers. Once
installed, you will need to select a method of securing it. This doc will
explain the options you have for security and help you find the resources needed
to configure it according to your chosen method. More configuration details can
be found in the [DHE Configuration page](./configuration.md).

Specifically, installation requires completion of these steps, in order:

1. Acquire a license by purchasing DHE or requesting a trial license.
2. Install the commercially supported Docker Engine.
3. Install DHE
4. Add your license to your DHE instance

> **Note:** This initial release of DHE has limited access. To get access,
> you will need an account on [Docker Hub](https://hub.docker.com/). Once you're
> logged in to the Hub with your account, visit the
> [early access registration page](https://registry.hub.docker.com/earlyaccess/)
> and follow the steps there to get signed up.

## Licensing

In order to run DHE, you will need to acquire a license, either by purchasing
DHE or requesting a trial license. The license will be associated with your
Docker Hub account or Docker Hub organization (so if you don't have an account,
you'll need to set one up, which can be done at the same time as your license
request). To get your license or start your trial, please contact our
[sales department](mailto:sales@docker.com). Upon completion of your purchase or
request, you will receive an email with further instructions for licensing your
copy of DHE.

## Prerequisites

DHE 1.0.1 requires the following:

* Commercially supported Docker Engine 1.6.1 or later running on an
Ubuntu 14.04 LTS, RHEL 7.1 or RHEL 7.0 host. (See below for instructions on how
to install the commercially supported Docker Engine.)

> **Note:** In order to remain in compliance with your DHE support agreement,
> you must use the current version of commercially supported Docker Engine.
> Running the regular, open source version of Engine is **not** supported.

* Your Docker daemon needs to be listening to the Unix socket (the default) so
that it can be bind-mounted into the DHE management containers, allowing
DHE to manage itself and its updates. For this reason, your DHE host will also
need internet connectivity so it can access the updates.

* Your host also needs to have TCP ports `80` and `443` available for the DHE
container port mapping.

* You will also need the Docker Hub user-name and password used when obtaining
the DHE license (or the user-name of an administrator of the Hub organization
that obtained an Enterprise license).

## Installing the Commercially Supported Docker Engine

Since DHE is installed using Docker, the commercially supported Docker Engine
must be installed first. This is done with an RPM or DEB repository, which you
set up using a Bash script downloaded from the [Docker Hub](https://hub.docker.com).

### Download the commercially supported Docker Engine installation script

To download the commercially supported Docker Engine Bash installation script,
log in to the [Docker Hub](https://hub.docker.com) with the user-name used to
obtain your license . Once you're logged in, go to the
["Enterprise Licenses"](https://registry.hub.docker.com/account/licenses/) page
in your Hub account's "Settings" section.

Select your intended host operating system from the "Download CS Engine" drop-
down at the top right of the page and then, once the Bash setup script is
downloaded, follow the steps below appropriate for your chosen OS.

![Docker Hub Docker engine install dropdown](../assets/docker-hub-org-enterprise-license-CSDE-dropdown.png)

### RHEL 7.0/7.1 installation

First, copy the downloaded Bash setup script to your RHEL host. Next, run the
following to install commercially supported Docker Engine and its dependencies,
and then start the Docker daemon:

```
$ sudo yum update && sudo yum upgrade
$ chmod 755 docker-cs-engine-rpm.sh
$ sudo ./docker-cs-engine-rpm.sh
$ sudo yum install docker-engine-cs
$ sudo systemctl enable docker.service
$ sudo systemctl start docker.service
```

In order to simplify using Docker, you can get non-sudo access to the Docker
socket by adding your user to the `docker` group, then logging out and back in
again:

```
$ sudo usermod -a -G docker $USER
$ exit
```

> **Note**: you may need to reboot your server to update its RHEL kernel.

### Ubuntu 14.04 LTS installation

First, copy the downloaded Bash setup script to your Ubuntu host. Next, run the
following to install commercially supported Docker Engine and its dependencies:

```
$ sudo apt-get update && sudo apt-get upgrade
$ sudo apt-get install -y linux-image-extra-virtual
$ sudo reboot
$ chmod 755 docker-cs-engine-deb.sh
$ sudo ./docker-cs-engine-deb.sh
$ sudo apt-get install docker-engine-cs
```
Lastly, confirm Docker is running with `sudo service docker start`.

In order to simplify using Docker, you can get non-sudo access to the Docker
socket by adding your user to the `docker` group, then logging out and back in
again:

```
$ sudo usermod -a -G docker $USER
$ exit
```

> **Note**: you may need to reboot your server to update its LTS kernel.

## Upgrading the Commercially Supported Docker Engine

CS Docker Engine 1.6.1 contains fixes to security vulnerabilities,
  and customers should upgrade to it immediately.

> **Note**: If you have CS Docker Engine 1.6.0 installed, it must be upgraded;
  however, due to compatibility issues, [DHE must be upgraded](#upgrading-docker-hub-enterprise)
  first.

The CS Docker Engine installation script set up the RHEL/Ubuntu package repositories,
so upgrading the Engine only requires you to run the update commands on your server.

### RHEL 7.0/7.1 upgrade

The following commands will stop the running DHE, upgrade CS Docker Engine,
and then start DHE again:

```
    $ sudo bash -c "$(sudo docker run dockerhubenterprise/manager stop)"
    $ sudo yum update
    $ sudo systemctl daemon-reload && sudo systemctl restart docker
    $ sudo bash -c "$(sudo docker run dockerhubenterprise/manager start)"
```

### Ubuntu 14.04 LTS upgrade

The following commands will stop the running DHE, upgrade CS Docker Engine,
and then start DHE again:

```
    $ sudo bash -c "$(sudo docker run dockerhubenterprise/manager stop)"
    $ sudo apt-get update && sudo apt-get dist-upgrade docker-engine-cs
    $ sudo bash -c "$(sudo docker run dockerhubenterprise/manager start)"
```

## Installing Docker Hub Enterprise

Once the commercially supported Docker Engine is installed, you can install DHE
itself. DHE is a self-installing application built and distributed using Docker
and the [Docker Hub](https://registry.hub.docker.com/). It is able to restart
and reconfigure itself using the Docker socket that is bind-mounted to its
container.

Start installing DHE by running the "dockerhubenterprise/manager" container:

```
	$ sudo bash -c "$(sudo docker run dockerhubenterprise/manager install)"
```

> **Note**: `sudo` is needed for `dockerhubenterprise/manager` commands to
> ensure that the Bash script is run with full access to the Docker host.

You can also find this command on the "Enterprise Licenses" section of your Hub
user profile. The command will execute a shell script that creates the needed
directories and then runs Docker to pull DHE's images and run its containers.

Depending on your internet connection, this process may take several minutes to
complete.

A successful installation will pull a large number of Docker images and should
display output similar to:

```
$ sudo bash -c "$(sudo docker run dockerhubenterprise/manager install)"
Unable to find image 'dockerhubenterprise/manager:latest' locally
Pulling repository dockerhubenterprise/manager
c46d58daad7d: Pulling image (latest) from dockerhubenterprise/manager
c46d58daad7d: Pulling image (latest) from dockerhubenterprise/manager
c46d58daad7d: Pulling dependent layers
511136ea3c5a: Download complete
fa4fd76b09ce: Pulling metadata
fa4fd76b09ce: Pulling fs layer
ff2996b1faed: Download complete
...
fd7612809d57: Pulling metadata
fd7612809d57: Pulling fs layer
fd7612809d57: Download complete
c46d58daad7d: Pulling metadata
c46d58daad7d: Pulling fs layer
c46d58daad7d: Download complete
c46d58daad7d: Download complete
Status: Downloaded newer image for dockerhubenterprise/manager:latest
Unable to find image 'dockerhubenterprise/manager:1.0.0_8ce62a61e058' locally
Pulling repository dockerhubenterprise/manager
c46d58daad7d: Download complete
511136ea3c5a: Download complete
fa4fd76b09ce: Download complete
1c8294cc5160: Download complete
117ee323aaa9: Download complete
2d24f826cb16: Download complete
33bfc1956932: Download complete
48f0dd6c9414: Download complete
65c30f72ecb2: Download complete
d4b29764d0d3: Download complete
5654f4fe5384: Download complete
9b9faa6ecd11: Download complete
0c275f56ca5c: Download complete
ff2996b1faed: Download complete
fd7612809d57: Download complete
Status: Image is up to date for dockerhubenterprise/manager:1.0.0_8ce62a61e058
INFO  [1.0.0_8ce62a61e058] Attempting to connect to docker engine dockerHost="unix:///var/run/docker.sock"
INFO  [1.0.0_8ce62a61e058] Running install command
<...output truncated...>
Creating container docker_hub_enterprise_load_balancer with docker daemon unix:///var/run/docker.sock
Starting container docker_hub_enterprise_load_balancer with docker daemon unix:///var/run/docker.sock
Bringing up docker_hub_enterprise_log_aggregator.
Creating container docker_hub_enterprise_log_aggregator with docker daemon unix:///var/run/docker.sock
Starting container docker_hub_enterprise_log_aggregator with docker daemon unix:///var/run/docker.sock
$ docker ps
CONTAINER ID        IMAGE                                                   COMMAND                CREATED             STATUS         PORTS                                      NAMES
0168f37b6221        dockerhubenterprise/log-aggregator:1.0.0_8ce62a61e058   "log-aggregator"       4 seconds ago       Up 4 seconds                                              docker_hub_enterprise_log_aggregator
b51c73bebe8b        dockerhubenterprise/nginx:1.0.0_8ce62a61e058            "nginxWatcher"         4 seconds ago       Up 4 seconds   0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp   docker_hub_enterprise_load_balancer
e8327864356b        dockerhubenterprise/admin-server:1.0.0_8ce62a61e058     "server"               5 seconds ago       Up 5 seconds   80/tcp                                     docker_hub_enterprise_admin_server
52885a6e830a        dockerhubenterprise/auth_server:alpha-a5a2af8a555e      "garant --authorizat   6 seconds ago       Up 5 seconds   8080/tcp
```

Once this process completes, you should be able to manage and configure your DHE
instance by pointing your browser to `https://<host-ip>/`.

Your browser will warn you that this is an unsafe site, with a self-signed,
untrusted certificate. This is normal and expected; allow this connection
temporarily.

### Setting the DHE Domain Name

The DHE Administrator site will also warn that the "Domain Name" is not set. Go
to the "Settings" tab, and set the "Domain Name" to the full host-name of your
DHE server.
Hitting the "Save and Restart DHE Server" button will generate a new certificate, which will be used
by both the DHE Administrator web interface and the DHE Registry server.

After the server restarts, you will again need to allow the connection to the untrusted DHE web admin site.

![http settings page</admin/settings#http>](../assets/admin-settings-http-unlicensed.png)

Lastly, you will see a warning notifying you that this instance of DHE is
unlicensed. You'll correct this in the next step.

### Add your license

The DHE registry services will not start until you add your license.
To do that, you'll first download your license from the Docker Hub and then
upload it to your DHE web admin server. Follow these steps:

1. If needed, log back into the [Docker Hub](https://hub.docker.com)
   using the user-name you used when obtaining your license. Go to "Settings" (in
   the menu under your user-name, top right) to get to your account settings, and
   then click on "Enterprise Licenses" in the side bar at left.

2. You'll see a list of available licenses. Click on the download button to
   obtain the license file you'd like to use.
   ![Download DHE license](../assets/docker-hub-org-enterprise-license.png)

3. Next, go to your DHE instance in your browser and click on the Settings tab
   and then the "License" tab. Click on the "Upload license file" button, which
   will open a standard file browser. Locate and select the license file you
   downloaded in step 2, above. Approve the selection to close the dialog.
   ![http settings page</admin/settings#license>](../assets/admin-settings-license.png)

4. Click the "Save and Restart DHE" button, which will quit DHE and then restart it, registering
   the new license.

5. Verify the acceptance of the license by confirming that the "unlicensed copy"
warning is no longer present.

### Securing DHE

Securing DHE is **required**. You will not be able to push or pull from DHE until you secure it.

There are several options and methods for securing DHE. For more information,
see the [configuration documentation](./configuration.md#security)

### Using DHE to push and pull images

Now that you have DHE configured with a "Domain Name" and have your client
Docker daemons configured with the required security settings, you can test your
setup by following the instructions for
[Using DHE to Push and pull images](./userguide.md#using-dhe-to-push-and-pull-images).

### DHE web interface and registry authentication

By default, there is no authentication set on either the DHE web admin
interface or the DHE registry. You can restrict access using an in-DHE
configured set of users (and passwords), or you can configure DHE to use LDAP-
based authentication.

See [DHE Authentication settings](./configuration.md#authentication) for more
details.

## Upgrading Docker Hub Enterprise

DHE has been designed to allow on-the-fly software upgrades. Start by
clicking on the "System Health" tab. In the upper, right-hand side of the
dashboard, below the navigation bar, you'll see the currently installed version
(e.g., `Current Version: 0.1.12345`).

If your DHE instance is the latest available, you will also see the message:
"System Up to Date."

If there is an upgrade available, you will see the message "System Update
Available!" alongside a button labeled "Update to Version X.XX". To upgrade, DHE
will pull new DHE container images from the Docker Hub. If you have not already
connected to Docker Hub, DHE will prompt you to log in.

The upgrade process requires a small amount of downtime to complete. To complete
the upgrade, DHE will:
* Connect to the Docker Hub to pull new container images with the new version of
DHE.
* Deploy those containers
* Shut down the old containers
* Resolve any necessary links/urls.

Assuming you have a decent internet connection, the entire upgrade process
should complete within a few minutes.

You should now [upgrade CS Docker Engine](#upgrading-the-commercially-supported-docker-engine).

> **Note**: If Docker engine is upgraded first (DHE 1.0.0 on CS Docker Engine 1.6.1),
> DHE can still be upgraded from the command line by running:
>
> `sudo bash -c "$(sudo docker run dockerhubenterprise/manager:1.0.0 upgrade 1.0.1)"`

## Next Steps

For information on configuring DHE for your environment, take a look at the
[Configuration instructions](./configuration.md).

