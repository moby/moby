Docker: a self-sufficient runtime for linux containers
======================================================

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
