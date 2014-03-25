# Docker Getting Started: Understanding Docker
----------------------------------------------------------------------

*What is Docker? What makes it great?*

## Introduction
----------------------------------------------------------------------

Dealing with servers, development machines or even personal computers in 
regards of running applications securely is a huge challenge. If you add 
portability, or even consistency to the question, it becomes even larger, 
something almost impossible to tackle correctly and properly. This rule
is 
applicable not only for individuals or small teams, but even to large 
companies and their IT departments.

The open-source Docker project provides free tools that users of all
levels and backgrounds can make-use-of to overcome a countless amount of 
such difficulties. Problems, ranging from virtual-machine performance 
issues and overheads to shipping software  can be all solved with Docker 
- easily. The project is growing rapidly with more features being added 
- every day -- and it can do tonnes of good for *you*.

### Table Of Contents
----------------------------------------------------------------------

1. Understanding Docker
    1. A Short History
    2. What Are Containers?
    3. How Does It Work?
2. Docker's Features
    1. User Centric (Simple to use)
    2. Portable And Free From Lock-Ins (Agnostic)
    3. Lightweight (Use less RAM)
    4. Accommodating (Run anything)
    5. Plays Well With Others (Run anywhere)
    6. Responsible (Manages and limits resources)
    7. Social (Share containers and images)
3. Docker Compared Against Virtual-Machines
    1. What Are Virtual-Machines?
    2. How Do Virtual-Machines Work?
    3. How Does Docker Compare?
    4. Summary
4. Docker Use Cases
    1. For Developers
    2. For System Administrators And Operators
    3. For Regular Computer Users
5. Where To Go From Here

## Understanding Docker
----------------------------------------------------------------------

Docker offers the user a set of tools that provide an extremely simple way to 
run almost any application (e.g. Memcached, Nginx, PostgreSQL etc.) securely,
isolated from any other found on the same system. Unlike the virtual-machines
(VMs), many Docker containers can exist and work simultaniously, together, even 
on a Virtual Private Server (VPS) -- all thanks to some brilliant Linux kernel
features with the complexities taken away. In deed, using Docker means using
plain English (e.g. `docker run` `[base-image-name]` `[application name]`).

A majority, if not all important features of virtual-machines that are required by
users come readily available with Docker and its containers. Furthermore,
Docker provides a very large amount of additional functionality that have long
been wanted and desired from the VMs, but are yet to be obtained. 

**Note:** Docker itself is *shipped* with the Apache 2.0 license and it is 
completely open-source -- *the pun? very much intended*.

### A Short History
----------------------------------------------------------------------

As a project, Docker's origins go back to a company's desire to provide the
most compact and successful solution to run any application (i.e. a platform).
Initially created to power the core technology behind the Platform-as-a-Service
(PaaS) provider [dotCloud](https://www.dotcloud.com), in March '13, Docker
was open-sourced and it has become one of the most popular and widely
contributed projects ever since.

### What Are Containers?
----------------------------------------------------------------------

Simply put, a Docker container is a simple system directory - inside which
literally everything that is needed for an application to run resides. 
In fact,as we have just mentioned, a base Linux image (e.g. Ubuntu, CentOS 
etc.) isalways used as the foundation. Each command executed forms a new 
layer on topof the base, hence creating a new image that can be used for 
another container.

> **Tip:** You can start a container with base Ubuntu, update it (i.e. 
> add a 2nd layer), install some libraries (i.e. then a 3rd layer) and 
> save your progress to use as a base for new ones - or you can boot-strap 
> it by setting up your own web-application and run a hundred containers 
> to horizontally scale with ease, on any machine with Docker installed.

### How Does It Work?
----------------------------------------------------------------------

Docker comes with two main tools and everything works with plain English. 
Together, the `docker` *client*, commanding the `docker` *daemon*, handle 
the task of creating, building and managing containers. Almost any application 
can runsecurely inside a container, and they are hardware and platform agnostic. 
Also,it should be mentioned that containers can be automatically created 
using the third main element of Docker: the *Dockerfile*.

> **Tip:** You can consider a *daemon* a program running in the background.
Most server applications, such as Apache or Nginx, or even databases, run like
daemons. *Clients* on the other hand, connect to daemons and tell them what to
do. This can all be done remotely (i.e. `client -> server`) as well.

The application works by taking advantage of certain Linux kernel features 
such as the `kernel namespaces` and `control groups`. Therefore, Docker can 
be considered a layer of extraction. However, as the project continues to 
grow with such momentum, even basics are being patched or developed anew 
for the greater good.

> **Note:** If you would like to learn more about the underlying technology, why not
jump to [Page 2](2_technology.md) where we talk about them? You can always come back here
to continue learning about Docker's features and what makes it different.

## Docker's Features
----------------------------------------------------------------------

In order to get a truly good grasp of Docker's full range of capabilities, one
would need to take a look at its [User's Manual](http://docs.docker.io). Here,
we will just try to summarise some good points with actual examples to 
give an overall idea of what can be accomplished using Docker.

### User Centric (Simple to use)
----------------------------------------------------------------------

*Docker is made for humans.*

Docker is built from the ground up to serve users - of all levels.

Any complexity you have ever come across when you wanted to achieve something
great in regards to secure application containment can be overcome using
Docker.

Regardless of your level, you can start benefiting - today! Any 
application you want to deploy, no matter how big or small, can be 
"dockerised" (and it should be, really).

**This translates to:**

 - Getting started is as easy as it could ever be;
 - Being able to use Docker in a matter of hours, not days or weeks;
 - Making your applications and servers lighter and more secure immediately.

**Example:**

i.e. `docker run [Image Name] [Application Name]`

### Portable And Free From Lock-Ins (Agnostic)
----------------------------------------------------------------------

*Dockerise And Go!*

Docker containers are highly portable and vendor lock-in free.

Any machine, be it bare-metal (i.e. actual, physical computer) or 
virtualised (i.e. a VPS), can run any Docker container. The sole requirement is to have Docker installed.

Imagine a machine snapshot saved in a folder - those are Docker containers.
They can be easily carried over to any other system and run the exact same 
way -- *as zipped and compressed archives* (i.e. a *tarball*).

**This translates to:**

 - Reliability;
 - Freeing your applications out of the dependency-hell;
 - A natural guarantee of how things will work, anywhere.

**Example:**

i.e. `docker save [image name] > [repository name].tar`  
i.e. `docker load [repository name].tar`

### Lightweight (Use less RAM)
----------------------------------------------------------------------

*No more resources waste.*

Containers are light, in fact, they are extremely light. Unlike VMs, Docker
does not need to anything other than what the actual process requires to run.

This translates to:

 - Ability to deploy an astonishing amount of applications on a single system;
 - Literally lightening fast start up times and reduced system over-heads.

### Accommodating (Run anything)
----------------------------------------------------------------------

*An amazing host! (again, pun intended.)*

Incase you might have missed: Even on a VPS you can run a hundred Dockerised 
applications - all isolated, contained and securely.

**This translates to:**

 - Ability to run a wide range of applications;
 - Ability to deploy reliably without repeating yourself.
 
### Plays Well With Others (Run anywhere)
----------------------------------------------------------------------

*A wonderful guest.*

Today, it is possible to install and use Docker almost anywhere. Even 
on non-Linux systems such as Windows or Mac OS X thanks to `boot2docker` 
initiative.

**This translates to running Docker (therefore containers) anywhere:**

 - **Linux Computers:**  
 Ubuntu, CentOS / RHEL, Fedora, Gentoo, openSUSE and more.
 - **Infrastructure-as-a-Service:**  
 Amazon AWS, Google GCE, Rackspace Cloud and probably, your favourite IaaS.
 
### Responsible (Manages and limits resources)
----------------------------------------------------------------------

*A tool that you can trust.*

Docker does not just bring you a set of tools to isolate and run 
applications. It also allows you to put in restraints and set up constraints -- *and it keeps trac*k.

**This translates to:**

 - Fine tuning available resources for each application;
 - Allocating memory or CPU intelligently to make most of your environment;
 
Without dealing with complicated commands or third party applications.

**Example:**

i.e. `docker run .. -m [Memory in MBs]m -c [CPU Share]`

### Social (Share containers and images)
----------------------------------------------------------------------

*Docker knows that No Man Is an Island.*

Each container you create consists of layers of additional data added on 
top of the base. A base can be any image.

Docker comes with a public index with the ability to run your own private 
one to share any snapshot of a container taken at any time -- and it's 
*free*. When you start using Docker, you can get a good portion of all 
existing popular tools with a single command and start using them securely 
inside a container.

**This translates to:**

 - No more wasting time building everything from scratch;
 - Easily save your application stack's *without* waiting 15 to 60 minutes;
 - Share and benefit with/from the rest of the Docker community.

**Example:**

i.e. `docker push [Container ID] [username]/[chosen repo. name]`

## Docker Compared Against Virtual-Machines
----------------------------------------------------------------------

> I suppose it is tempting, if the *only* tool you have is a hammer, to 
> treat *everything* as if it were a nail.  
> - **_Abraham Maslow_**

Virtual-machines have their place and they will continue to do so for a 
long time. They fulfil a niche, if not a very large gap, in the IT industry. 
However, as you will see, when compared to containers, they are being
over-used -- and a lot of the time, *very unnecessarily*.

### What Are Virtual-Machines?
----------------------------------------------------------------------

Virtual-machines (VMs) are applications that provide the full experience 
of a computer environment through *emulation*. Depending on the 
requirements, different levels of virtualisation can be obtained, some 
technologies reserving resources for the sole use of the VM, and some, 
just letting it 
access as needed.

In all cases, working with VMs boils down to trying to make most of the 
available hardware or containing (and porting) application collections 
consistently between different instalments. 

For a majority of users, VMs come in handy when they want to rent a server 
to deploy or run applications. The need of emulating a certain configuration 
for testing purposes, or, keeping things separated can be considered good 
examples for use cases as well.

However, VMs are far from perfect. Actually an over-kill for many, they 
require a lot of resources simply to run in first place -- and they are 
extremely heavy to deal with.

### How Do Virtual-Machines Work?
----------------------------------------------------------------------

There are a lot of different ways to obtain a multi-VM set up on a 
single physical server. They all rely on an underlying system to prepare 
the host (i.e. the actual computer) to run them at certain levels of 
isolation. Since VMs rely on emulation of a full operating-system, there 
are huge over-head issues involved.

### How Does Docker Compare?
----------------------------------------------------------------------

Docker containers do not try to emulate anything. Containers artfully 
contain whatever you choose to put and run inside -- and this can be a 
fully automated process.

Since there is no heavy emulation and overheads, containers become 
light to carry over, to work with, to ship or share.

Contradictory to VM status-quo, containers are not vendor or platform 
specific. You can swap your provider as you like and use your own 
development machine to create one to later deploy online.

### Summary
----------------------------------------------------------------------

**Docker containers are:**

 - Easy on the resources;
 - Extremely light to deal with;
 - Do not come with overheads;
 - Very easy to work with;
 - Agnostic in essence;
 - Can work *on* virtual servers;
 - Secure and isolated; 
 - *Artful*, *social*, *fun*, and;
 - Powerful sand-boxes.

**Docker containers are not:**

 - Hardware or OS emulators;
 - Nor heavy on the resources.
 - Platform, software or language dependant.

## Docker Use Cases
----------------------------------------------------------------------

Essentially, Docker is a tool. It is not basic - but an excellent base.
A lot can be done and its limits go as far as your application containment 
needs.

Docker and containers are:

### For Developers
----------------------------------------------------------------------

 - **Developed with developers in mind:**  
 Build, test and ship applications with nothing but Docker and lean 
 containers.
 - **Re-usable building blocks to create more:**  
 Any container can be a base for the next, and each command a new block.
 - **Automatically build-able:**  
 It has never been this easy to instruct and build - *anything*.
 - **Built upon:**  
 Numerous third party tools and platforms are built on Docker, for your containers.

### For Development And Operations
----------------------------------------------------------------------

 - **Efficient DevOps lifecycle:**  
 Operations and developments are consistent, repeatable and reliable.
 - **Balanced environments:**  
 Processes between development, testing and production are levelled.
 - **Improvements on speed and integration:**  
 Containers are almost nothing more than isolated and securely kept 
 processes.
 - **Lowered costs of infrastructure:**  
 Containers are lightweight and heavy on resources compared to VMs.
 - **Portable configurations:**  
 Issues and overheads with dealing with configurations and systems are eliminated.

### For Regular Computer Users
----------------------------------------------------------------------

 - **Increased security without performance loss:**  
 Replacing VMs with containers provide security without additional 
 hardware (or software).
 - **Portable:**  
 You can securely carry around applications, the exact way they exist.

## Where To Go From Here
----------------------------------------------------------------------

### Learn About Docker's Part And The Underlying Technology
----------------------------------------------------------------------

Visit [Page 2](2_technology.md) of our Getting Started manual.

### Get Practical And Learn How To Use Docker Straight Away
----------------------------------------------------------------------

Visit [Page 3](3_dockerfile.md) of our Getting Started manual.

### Get The Product And Go Hands-On
----------------------------------------------------------------------

Visit [Page 4](4_installation.md) of our Getting Started manual.

### Get The Whole Story
----------------------------------------------------------------------

[https://www.docker.io/the_whole_story/](https://www.docker.io/the_whole_story/)