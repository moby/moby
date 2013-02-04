Docker: a self-sufficient runtime for linux containers
======================================================

<img src="http://bricks.argz.com/bricksfiles/lego/07000/7823/012.jpg"/>

Docker is a runtime for Standard Containers. More specifically, it is a daemon which automates the creation of and deployment of linux Standard Containers (SCs) via a remote API.

Standard Containers are a fundamental unit of software delivery, in much the same way that shipping containers (http://bricks.argz.com/ins/7823-1/12) are a fundamental unit of physical delivery.


1. STANDARD OPERATIONS
----------------------

Just like shipping containers, Standard Containers define a set of STANDARD OPERATIONS. Shipping containers can be lifted, stacked, locked, loaded, unloaded and labelled. Similarly, standard containers can be started, stopped, copied, snapshotted, downloaded, uploaded and tagged.


2. CONTENT-AGNOSTIC
------------------

Just like shipping containers, Standard Containers are CONTENT-AGNOSTIC: all standard operations have the same effect regardless of the contents. A shipping container will be stacked in exactly the same way whether it contains Vietnamese powder coffe or spare Maserati parts. Similarly, Standard Containers are started or uploaded in the same way whether they contain a postgres database, a php application with its dependencies and application server, or Java build artifacts.


3. INFRASTRUCTURE-AGNOSTIC
--------------------------

Both types of containers are INFRASTRUCTURE-AGNOSTIC: they can be transported to thousands of facilities around the world, and manipulated by a wide variety of equipment. A shipping container can be packed in a factory in Ukraine, transported by truck to the nearest routing center, stacked onto a train, loaded into a German boat by an Australian-built crane, stored in a warehouse at a US facility, etc. Similarly, a standard container can be bundled on my laptop, uploaded to S3, downloaded, run and snapshotted by a build server at Equinix in Virginia, uploaded to 10 staging servers in a home-made Openstack cluster, then sent to 30 production instances across 3 EC2 regions.


4. DESIGNED FOR AUTOMATION
--------------------------

Because they offer the same standard operations regardless of content and infrastructure, Standard Containers, just like their physical counterpart, are extremely well-suited for automation. In fact, you could say automation is their secret weapon.

Many things that once required time-consuming and error-prone human effort can now be programmed. Before shipping containers, a bag of powder coffee was hauled, dragged, dropped, rolled and stacked by 10 different people in 10 different locations by the time it reached its destination. 1 out of 50 disappeared. 1 out of 20 was damaged. The process was slow, inefficient and cost a fortune - and was entirely different depending on the facility and the type of goods.

Similarly, before Standard Containers, by the time a software component ran in production, it had been individually built, configured, bundled, documented, patched, vendored, templated, tweaked and instrumented by 10 different people on 10 different computers. Builds failed, libraries conflicted, mirrors crashed, post-it notes were lost, logs were misplaced, cluster updates were half-broken. The process was slow, inefficient and cost a fortune - and was entirely different depending on the language and infrastructure provider.


5. INDUSTRIAL-GRADE DELIVERY
----------------------------

There are 17 million shipping containers in existence, packed with every physical good imaginable. Every single one of them can be loaded on the same boats, by the same cranes, in the same facilities, and sent anywhere in the World with incredible efficiency. It is embarrassing to think that a 30 ton shipment of coffee can safely travel half-way across the World in *less time* than it takes a software team to deliver its code from one datacenter to another sitting 10 miles away.

With Standard Containers we can put an end to that embarrassment, by making INDUSTRIAL-GRADE DELIVERY of software a reality.


Setup instructions
==================

Supported hosts
---------------

Right now, the officially supported hosts are:
* Ubuntu 12.10 (quantal)

Hosts that might work with slight kernel modifications, but are not officially supported:
* Ubuntu 12.04 (precise)

Step by step host setup
-----------------------

1. Set up your host of choice on a physical / virtual machine
2. Assume root identity on your newly installed environment (`sudo -s`)
3. Type the following commands:

        apt-get update
        apt-get install lxc wget
        debootstrap --arch=amd64 quantal /var/lib/docker/images/ubuntu/

4. Download the latest version of the [docker binaries](https://dl.dropbox.com/u/20637798/docker.tar.gz) (`wget https://dl.dropbox.com/u/20637798/docker.tar.gz`)
5. Extract the contents of the tar file `tar -xf docker.tar.gz`
6. Launch the docker daemon `./dockerd`


Client installation
-------------------

4. Download the latest version of the [docker binaries](https://dl.dropbox.com/u/20637798/docker.tar.gz) (`wget https://dl.dropbox.com/u/20637798/docker.tar.gz`)
5. Extract the contents of the tar file `tar -xf docker.tar.gz`
6. You can now use the docker client binary `./docker`. Consider adding it to your `PATH` for simplicity.

Vagrant Usage
-------------

1. Install Vagrant from http://vagrantup.com
2. Run `vagrant up`. This will take a few minutes as it does the following:
    - Download Quantal64 base box
    - Kick off Puppet to do:
        - Download & untar most recent docker binary tarball to vagrant homedir.
        - Debootstrap to /var/lib/docker/images/ubuntu.
        - Install & run dockerd as service.
        - Put docker in /usr/local/bin.
        - Put latest Go toolchain in /usr/local/go.

Sample run output:

```bash
$ vagrant up
[default] Importing base box 'quantal64'...
[default] Matching MAC address for NAT networking...
[default] Clearing any previously set forwarded ports...
[default] Forwarding ports...
[default] -- 22 => 2222 (adapter 1)
[default] Creating shared folders metadata...
[default] Clearing any previously set network interfaces...
[default] Booting VM...
[default] Waiting for VM to boot. This can take a few minutes.
[default] VM booted and ready for use!
[default] Mounting shared folders...
[default] -- v-root: /vagrant
[default] -- manifests: /tmp/vagrant-puppet/manifests
[default] -- v-pp-m0: /tmp/vagrant-puppet/modules-0
[default] Running provisioner: Vagrant::Provisioners::Puppet...
[default] Running Puppet with /tmp/vagrant-puppet/manifests/quantal64.pp...
stdin: is not a tty
notice: /Stage[main]//Node[default]/Exec[apt_update]/returns: executed successfully

notice: /Stage[main]/Docker/Exec[fetch-docker]/returns: executed successfully
notice: /Stage[main]/Docker/Package[lxc]/ensure: ensure changed 'purged' to 'present'
notice: /Stage[main]/Docker/Exec[fetch-go]/returns: executed successfully

notice: /Stage[main]/Docker/Exec[copy-docker-bin]/returns: executed successfully
notice: /Stage[main]/Docker/Exec[debootstrap]/returns: executed successfully
notice: /Stage[main]/Docker/File[/etc/init/dockerd.conf]/ensure: defined content as '{md5}78a593d38dd9919af14d8f0545ac95e9'

notice: /Stage[main]/Docker/Service[dockerd]/ensure: ensure changed 'stopped' to 'running'

notice: Finished catalog run in 329.74 seconds
```
3. When this has successfully completed, you should be albe to get into your new system with `vagrant ssh` and use `docker`:

```bash
$ vagrant ssh
Welcome to Ubuntu 12.10 (GNU/Linux 3.5.0-17-generic x86_64)

 * Documentation:  https://help.ubuntu.com/

Last login: Sun Feb  3 19:37:37 2013
vagrant@vagrant-ubuntu-12:~$ DOCKER=localhost:4242 docker help
Usage: docker COMMAND [arg...]

A self-sufficient runtime for linux containers.

Commands:
    run       Run a command in a container
    ps        Display a list of containers
    pull      Download a tarball and create a container from it
    put       Upload a tarball and create a container from it
    rm        Remove containers
    wait      Wait for the state of a container to change
    stop      Stop a running container
    logs      Fetch the logs of a container
    diff      Inspect changes on a container's filesystem
    commit    Save the state of a container
    attach    Attach to the standard inputs and outputs of a running container
    info      Display system-wide information
    tar       Stream the contents of a container as a tar archive
    web       Generate a web UI
    attach    Attach to a running container
```
    
