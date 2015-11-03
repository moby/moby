<!--[metadata]>
+++
title = "Get started with multi-host networking"
description = "Use overlay for multi-host networking"
keywords = ["Examples, Usage, network, docker, documentation, user guide, multihost, cluster"]
[menu.main]
parent = "smn_networking"
weight=-3
+++
<![end-metadata]-->     

# Get started with multi-host networking

This article uses an example to explain the basics of creating a multi-host
network. Docker Engine supports multi-host-networking out-of-the-box through the
`overlay` network driver.  Unlike `bridge` networks overlay networks require
some pre-existing conditions before you can create one. These conditions are:

* A host with a 3.16 kernel version or higher.
* Access to a key-value store. Docker supports Consul, Etcd, and ZooKeeper (Distributed store) key-value stores.
* A cluster of hosts with connectivity to the key-value store.
* A properly configured Engine `daemon` on each host in the cluster.

Though Docker Machine and Docker Swarm are not mandatory to experience Docker
multi-host-networking, this example uses them to illustrate how they are
integrated. You'll use Machine to create both the the key-value store
server and the host cluster. This example creates a Swarm cluster.

## Prerequisites

Before you begin, make sure you have a system on your network with the latest
version of Docker Engine and Docker Machine installed. The example also relies
on VirtualBox. If you installed on a Mac or Windows with Docker Toolbox, you
have all of these installed already.

If you have not already done so, make sure you upgrade Docker Engine and Docker
Machine to the latest versions.


## Step 1: Set up a key-value store

An overlay network requires a key-value store. The key-value stores information
about the network state which includes discovery, networks, endpoints,
ip-addresses, and more. Docker supports Consul, Etcd, and ZooKeeper (Distributed
store) key-value stores. This example uses Consul.

1. Log into a system prepared with the prerequisite Docker Engine, Docker Machine, and VirtualBox software.

2. Provision a VirtualBox machine called `mh-keystore`.  

		$ docker-machine create -d virtualbox mh-keystore

	When you provision a new machine, the process adds Docker Engine to the
	host. This means rather than installing Consul manually, you can create an
	instance using the [consul image from Docker
	Hub](https://hub.docker.com/r/progrium/consul/). You'll do this in the next step.

3. Start a `progrium/consul` container running on the `mh-keystore` machine.

		$  docker $(docker-machine config mh-keystore) run -d \
			-p "8500:8500" \
			-h "consul" \
			progrium/consul -server -bootstrap

	 You passed the `docker run` command the connection configuration using a bash
	 expansion `$(docker-machine config mh-keystore)`.  The client started a
	 `progrium/consul` image running in the `mh-keystore` machine. The server is called `consul`and is listening port `8500`.

4. Set your local environment to the `mh-keystore` machine.

		$  eval "$(docker-machine env mh-keystore)"

5. Run the `docker ps` command to see the `consul` container.

		$ docker ps
		CONTAINER ID        IMAGE               COMMAND                  CREATED             STATUS              PORTS                                                                            NAMES
		4d51392253b3        progrium/consul     "/bin/start -server -"   25 minutes ago      Up 25 minutes       53/tcp, 53/udp, 8300-8302/tcp, 0.0.0.0:8500->8500/tcp, 8400/tcp, 8301-8302/udp   admiring_panini

Keep your terminal open and move onto the next step.


## Step 2: Create a Swarm cluster

In this step, you use `docker-machine` to provision the hosts for your network.
At this point, you won't actually created the network. You'll create several
machines in VirtualBox. One of the machines will act as the Swarm master;
you'll create that first. As you create each host, you'll pass the Engine on
that machine options that are needed by the `overlay` network driver.

1. Create a Swarm master.

		$ docker-machine create \
		-d virtualbox \
		--swarm --swarm-image="swarm" --swarm-master \
		--swarm-discovery="consul://$(docker-machine ip mh-keystore):8500" \
		--engine-opt="cluster-store=consul://$(docker-machine ip mh-keystore):8500" \
		--engine-opt="cluster-advertise=eth1:2376" \
		mhs-demo0

	At creation time, you supply the Engine `daemon` with the ` --cluster-store` option. This option tells the Engine the location of the key-value store for the `overlay` network. The bash expansion `$(docker-machine ip mh-keystore)` resolves to the IP address of the Consul server you created in "STEP 1". The `--cluster-advertise` option advertises the machine on the network.

2. Create another host and add it to the Swarm cluster.

		$ docker-machine create -d virtualbox \
			--swarm --swarm-image="swarm:1.0.0-rc2" \
			--swarm-discovery="consul://$(docker-machine ip mh-keystore):8500" \
			--engine-opt="cluster-store=consul://$(docker-machine ip mh-keystore):8500" \
			--engine-opt="cluster-advertise=eth1:2376" \
		  mhs-demo1

3. List your machines to confirm they are all up and running.

		$ docker-machine ls
		NAME         ACTIVE   DRIVER       STATE     URL                         SWARM
		default               virtualbox   Running   tcp://192.168.99.100:2376   
		mh-keystore           virtualbox   Running   tcp://192.168.99.103:2376   
		mhs-demo0             virtualbox   Running   tcp://192.168.99.104:2376   mhs-demo0 (master)
		mhs-demo1             virtualbox   Running   tcp://192.168.99.105:2376   mhs-demo0

At this point you have a set of hosts running on your network. You are ready to create a multi-host network for containers using these hosts.


Leave your terminal open and go onto the next step.

## Step 3: Create the overlay Network

To create an overlay network

1. Set your docker environment to the Swarm master.

		$ eval $(docker-machine env --swarm mhs-demo0)

	Using the `--swarm` flag with `docker-machine` restricts the `docker` commands to Swarm information alone.

2. Use the `docker info` command to view the Swarm.

		$ docker info
		Containers: 3
		Images: 2
		Role: primary
		Strategy: spread
		Filters: affinity, health, constraint, port, dependency
		Nodes: 2
		mhs-demo0: 192.168.99.104:2376
		└ Containers: 2
		└ Reserved CPUs: 0 / 1
		└ Reserved Memory: 0 B / 1.021 GiB
		└ Labels: executiondriver=native-0.2, kernelversion=4.1.10-boot2docker, operatingsystem=Boot2Docker 1.9.0-rc1 (TCL 6.4); master : 4187d2c - Wed Oct 14 14:00:28 UTC 2015, provider=virtualbox, storagedriver=aufs
		mhs-demo1: 192.168.99.105:2376
		└ Containers: 1
		└ Reserved CPUs: 0 / 1
		└ Reserved Memory: 0 B / 1.021 GiB
		└ Labels: executiondriver=native-0.2, kernelversion=4.1.10-boot2docker, operatingsystem=Boot2Docker 1.9.0-rc1 (TCL 6.4); master : 4187d2c - Wed Oct 14 14:00:28 UTC 2015, provider=virtualbox, storagedriver=aufs
		CPUs: 2
		Total Memory: 2.043 GiB
		Name: 30438ece0915

	From this information, you can see that you are running three containers and 2 images on the Master.

3. Create your `overlay` network.

		$ docker network create --driver overlay my-net

	You only need to create the network on a single host in the cluster. In this case, you used the Swarm master but you could easily have run it on any host in the cluster.

4. Check that the network is running:

		$ docker network ls
		NETWORK ID          NAME                DRIVER
		412c2496d0eb        mhs-demo1/host      host                
		dd51763e6dd2        mhs-demo0/bridge    bridge              
		6b07d0be843f        my-net              overlay             
		b4234109bd9b        mhs-demo0/none      null                
		1aeead6dd890        mhs-demo0/host      host                
		d0bb78cbe7bd        mhs-demo1/bridge    bridge              
		1c0eb8f69ebb        mhs-demo1/none      null     

	Because you are in the Swarm master environment, you see all the networks on all Swarm agents. Notice that each `NETWORK ID` is unique.  The default networks on each engine and the single overlay network.  

5. Switch to each Swarm agent in turn and list the network.

		$ eval $(docker-machine env mhs-demo0)
		$ docker network ls
		NETWORK ID          NAME                DRIVER
		6b07d0be843f        my-net              overlay             
		dd51763e6dd2        bridge              bridge              
		b4234109bd9b        none                null                
		1aeead6dd890        host                host                
		$ eval $(docker-machine env mhs-demo1)
		$ docker network ls
		NETWORK ID          NAME                DRIVER
		d0bb78cbe7bd        bridge              bridge              
		1c0eb8f69ebb        none                null                
		412c2496d0eb        host                host                
		6b07d0be843f        my-net              overlay        

  Both agents reports it has the `my-net `network with the `6b07d0be843f` id.  You have a multi-host container network running!

##  Step 4: Run an application on your Network

Once your network is created, you can start a container on any of the hosts and it automatically is part of the network.

1. Point your environment to your `mhs-demo0` instance.

		$ eval $(docker-machine env mhs-demo0)

2. Start an Nginx server on `mhs-demo0`.

		$ docker run -itd --name=web --net=my-net --env="constraint:node==mhs-demo0" nginx

	This command starts a web server on the Swarm master.

3. Point your Machine environment to `mhs-demo1`

		$ eval $(docker-machine env mhs-demo1)

4. Run a Busybox instance and get the contents of the Ngnix server's home page.

		$ docker run -it --rm --net=my-net --env="constraint:node==mhs-demo1" busybox wget -O- http://web
		Unable to find image 'busybox:latest' locally
		latest: Pulling from library/busybox
		ab2b8a86ca6c: Pull complete
		2c5ac3f849df: Pull complete
		Digest: sha256:5551dbdfc48d66734d0f01cafee0952cb6e8eeecd1e2492240bf2fd9640c2279
		Status: Downloaded newer image for busybox:latest
		Connecting to web (10.0.0.2:80)
		<!DOCTYPE html>
		<html>
		<head>
		<title>Welcome to nginx!</title>
		<style>
		body {
				width: 35em;
				margin: 0 auto;
				font-family: Tahoma, Verdana, Arial, sans-serif;
		}
		</style>
		</head>
		<body>
		<h1>Welcome to nginx!</h1>
		<p>If you see this page, the nginx web server is successfully installed and
		working. Further configuration is required.</p>

		<p>For online documentation and support please refer to
		<a href="http://nginx.org/">nginx.org</a>.<br/>
		Commercial support is available at
		<a href="http://nginx.com/">nginx.com</a>.</p>

		<p><em>Thank you for using nginx.</em></p>
		</body>
		</html>
		-                    100% |*******************************|   612   0:00:00 ETA

## Step 5: Check external connectivity

As you've seen, Docker's built-in overlay network driver provides out-of-the-box
connectivity between the containers on multiple hosts within the same network.
Additionally, containers connected to the multi-host network are automatically
connected to the `docker_gwbridge` network. This network allows the containers
to have external connectivity outside of their cluster.

1. Change your environment to the Swarm agent.

		$ eval $(docker-machine env mhs-demo1)

2. View the `docker_gwbridge` network, by listing the networks.

		$ docker network ls
		NETWORK ID          NAME                DRIVER
		6b07d0be843f        my-net              overlay
		dd51763e6dd2        bridge              bridge
		b4234109bd9b        none                null
		1aeead6dd890        host                host
		e1dbd5dff8be        docker_gwbridge     bridge

3. Repeat steps 1 and 2 on the Swarm master.

		$ eval $(docker-machine env mhs-demo0)
		$ docker network ls
		NETWORK ID          NAME                DRIVER
		6b07d0be843f        my-net              overlay
		d0bb78cbe7bd        bridge              bridge
		1c0eb8f69ebb        none                null
		412c2496d0eb        host                host
		97102a22e8d2        docker_gwbridge     bridge

2. Check the Ngnix container's network interfaces.

		$ docker exec web ip addr
		1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default
		link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
		inet 127.0.0.1/8 scope host lo
		    valid_lft forever preferred_lft forever
		inet6 ::1/128 scope host
		    valid_lft forever preferred_lft forever
		22: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1450 qdisc noqueue state UP group default
		link/ether 02:42:0a:00:09:03 brd ff:ff:ff:ff:ff:ff
		inet 10.0.9.3/24 scope global eth0
		    valid_lft forever preferred_lft forever
		inet6 fe80::42:aff:fe00:903/64 scope link
		    valid_lft forever preferred_lft forever
		24: eth1: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default
		link/ether 02:42:ac:12:00:02 brd ff:ff:ff:ff:ff:ff
		inet 172.18.0.2/16 scope global eth1
		    valid_lft forever preferred_lft forever
		inet6 fe80::42:acff:fe12:2/64 scope link
		    valid_lft forever preferred_lft forever

	The `eth0` interface represents the container interface that is connected to
	the `my-net` overlay network. While the `eth1` interface represents the
	container interface that is connected to the `docker_gwbridge` network.

## Step 6: Extra Credit with Docker Compose

You can try starting a second network on your existing Swarm cluster using Docker Compose.

1. Log into the Swarm master.

2. Install Docker Compose.

3. Create a `docker-compose.yml` file.

4. Add the following content to the file.

		web:
			image: bfirsh/compose-mongodb-demo
			environment:
				- "MONGO_HOST=counter_mongo_1"
				- "constraint:node==swl-demo0"
			ports:
				- "80:5000"
		mongo:
			image: mongo

5. Save and close the file.

6. Start the application with Compose.

		$ docker-compose up --x-networking up -d

## Related information

* [Understand Docker container networks](dockernetworks.md)
* [Work with network commands](work-with-networks.md)
* [Docker Swarm overview](https://docs.docker.com/swarm)
* [Docker Machine overview](https://docs.docker.com/machine)
