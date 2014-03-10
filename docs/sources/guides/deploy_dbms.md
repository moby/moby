# How To Deploy Database Management Systems Using Docker
----------------------------------------------------------------------

## Introduction And Contents
----------------------------------------------------------------------

### Introduction
----------------------------------------------------------------------

Unless *reading-from* / *writing-to* disk, or, working with a relatively 
simple DB implementation such as SQLite is sufficient, you will want 
to use a fully-fledged database management system for all sorts of data 
operations with your applications.

In this example, we are going to see how to use Docker to consistently 
manage hosting and deployment processes of various database solutions 
(i.e. DBMSs) using *containers*. Our goal here is to guide you through, 
as we exhibit, how this exceptionally capable technology, with all of 
the complexities extracted, can help you to lessen the challenges of 
working with such advanced software applications as databases.

### Contents
----------------------------------------------------------------------

1. Understanding The Challenge
    1. Application Deployment
    2. Deploying And Managing Databases
2. Introducing Docker Into The Challenge
3. Building A Docker Container Step-by-Step
    1. Understanding The Workflow
    2. Instantiating A Container
    3. Setting Up A Container To Deploy Databases
    4. Installing Databases (PostgreSQL, MySQL, MongoDB etc.)
    5. Saving Your Database Container As A Docker Image
    6. Running Container(s) With Your Database Installation
4. Automating The Build Process Using A Dockerfile
    1. Understanding The Dockerfile
    2. Crating A Dockerfile To Automate Web Application Deployments
    3. Using A Dockerfile To Build Images And Run Containers
5. Questions & Ideas Which Might Pop Up In Your Mind

## Understanding The Challenge
----------------------------------------------------------------------

### Application Deployment
----------------------------------------------------------------------

When it comes to *applications*, deployment is generally used to cover 
the series of actions performed when making a higher-level application 
accessible to the intended user(s) and other clients alike.

In a very broad sense, the term *deployment* signifies something being 
put into service, ready to be operated.

Granted, the details of this notion is very much open to discussion and 
the term itself is rather subjective. Nonetheless, it is clear that for 
everyone, it means *getting up-and-running*. 

### Deploying And Managing Databases
----------------------------------------------------------------------

The process of deploying and managing databases is usually what those 
in charge of the task make of it: a very simple and basic one, or 
the complete opposite - quite a complex thing. This, of course, depends 
on the needs of the application(s) using the database, as well.

Given the wide array of different types of easily accessible database 
management system technologies available, it isn't hard to imagine a 
scenario whereby multiple NoSQL database servers, together with one 
or more relational database (i.e. RDBMS) instances are in use to power 
your application.

This translates to preparing one (or mostly like more than one) server 
with custom configurations for each one of the database applications 
deployed, with directories on the machine separated to support these 
systems -- together with a back-up, upgrade and fail-over system put 
in place for the rainy day.

## Introducing Docker Into The Challenge
----------------------------------------------------------------------

Docker, as a platform and application agnostic technology, offers the 
tools to ease and lessen the complexities of dealing with deployment 
and management of databases. This is achieved by using containers.

Containers provide isolated, highly portable and manageable bases to deploy and run any application -- including databases.

In case of NoSQL solutions that do not require a persistent storage 
array, using a container, built with your specifications, it becomes 
possible to run any number of instances, on any machine with Docker 
installed, by executing a single command (i.e. `docker run`).

For more advanced relational database systems (e.g. MySQL or Postgres), 
keeping the managed data permanently, without explicitly disposing, is 
a must. The ability to share volumes (i.e. parts of the storage on the 
host machine) makes it possible to back-up, upgrade, and cluster 
(multiple) database servers.

To summarise, Docker can be said to help you in the following ways:

1. **Consistency:**  
   Docker containers can be configured in anyway, just like a VM. This 
   provides an always-available snapshot of your database application 
   which can be run without the need to configure, or even upload any 
   additional file. It is up to you and your application's needs to 
   decide whether or not to attach, or, keep the data bundled with your 
   container(s) -- which are highly portable by nature.
2. **Simple deployments thanks to portability:**  
   Containers are highly portable. They only require system operators
   (or single developers) to push them to their servers to get up and 
   running -- or scale across any number of machines with all disk, 
   data and application configurations intact.
3. **Simple deployments thanks to automation:**  
   Docker containers can be built automatically to your application's 
   specification. Without investing time and effort on other tools, 
   or, by implementing Docker into your current workflow, it becomes 
   possible to manage highly-complex DBMSs very simply.
4. **High-availability:**  
   Since containerised (i.e. *dockerised*) applications run literally 
   in an instant, achieving high-availability without undergoing huge 
   technical (or financial) charges becomes a possibility for everyone 
   with little time spent on the initial set-up.
5. **Easy scaling:**  
   Using containers, duplicating instances running your application 
   can be increased (or decreased) by executing a single command for 
   each operation respectively. This brings the possibility to backup, 
   hot-upgrade or replicate the database data *like never before*.
6. **Enhanced security:**  
   Containers, and applications deployed inside, can be considered 
   like powerful sandboxes. They do not have outside access unless 
   specified and their reach stays within the boundaries you set.

Continuing in this example, we will now see how to treat a container 
like a virtual machine, connect to it, prepare it like a computer of 
its own, and then commit the end result to be used in the future -- 
to deploy / launch different database instances with different needs.

Next, we will see how to craft a *Dockerfile* script to automate image 
creation (i.e. the *build process*). This way, we are going to have a 
Docker container that will be built, step-by-step, fully automatically, 
configured to suit your application / deployment needs.

> **Note:** Please remember that for a majority of cases, using Docker 
> files to automate the creating of a container image is the way to go.
> Using a container like a VM and connecting to it (i.e. `attach`), is 
> an unnecessarily complex process. However, in our example we will go 
> over it to get an over-all feeling of how to work with Docker 
> containers. Please see our *Examples Section* for Dockerfile examples 
> on deploying web applications.

Let's begin!  

## Building A Docker Container Step-by-Step
----------------------------------------------------------------------

### Understanding The Workflow
----------------------------------------------------------------------

> **Note:** This section, and the next one, requires you to have Docker 
> installed, i.e. the `docker` client and the `docker` daemon ready.

Building a container step-by-step requires us to instantiate a container.
Afterwards, we need to build it to run the database server. In general, 
this process will consist of the following steps:

1. **Adding the repository links:**  
   Default base images are usually kept lean. Therefore, we need to 
   append the application repository links we need access to, in order 
   to download the database management system we want to use.
2. **Updating the default package manager sources:**  
   Once new repository links are added (e.g. Ubuntu's `universe` or 
   `EPEL` for RHEL-based distributions, or custom vendor repositories), 
   we need to update the packages.
3. **Downloading the basic tools we will need for management:**  
   Containers are absolutely isolated. In case of using a simple base 
   image, even for basic tasks, you will need some tools installed. 
   This means getting applications like `wget`, `nano`, or essential 
   build software (e.g. `make`) -- just like the older days of CentOS.
4. **Installing the database server:**  
   After getting all the tools, the next step will be installing the 
   database server itself.
5. **Configuring the database:**  
   Next, as expected, we will need to configure the database.
6. **Running the database:**  
   After everything is set, it is time to deploy and run the database.

### Instantiating A Container
----------------------------------------------------------------------

In order to work with Docker containers, the first thing to do is to 
create one. Container creation process consists of executing the `run` 
command (i.e. `docker run`) and specifying the name of a `base image` to 
use. Afterwards, we need to provide `docker` with options and settings, 
together with the application we would like to have it running. This is 
necessary for `docker` to instantiate the container.

**Note:** `docker run` command is not platform specific. You can use 
the below examples, to run containers with different Linux distributions.

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

In order to interact with the containers like a VM, we will have it 
running `bash` shell, a very common command-based user interface. 

Execute the following command to have a container running `bash`:

    docker run -i -t ubuntu /bin/bash

|| RHEL / CentOS

In order to interact with the containers like a VM, we will have it 
running `bash` shell, a very common command-based user interface.

Execute the following command to have a container running `bash`:

    docker run -i -t centos /bin/bash

|

> If the given base image is not found on the host machine where the 
`docker` daemon is running, it gets downloaded for use.

**Naming (i.e. tagging) a container:**

If you would like to name (i.e. `tag`) a container, you can use the 
`-name` parameter, e.g.:

    # Usage: docker run -name [name] [.. options] [image] [process]
    # Example:
    docker run -name my_container -i -t centos /bin/bash

**Leaving (i.e. detaching from) the container:**

Once you are attached to a container, in order to leave, you will 
need to run *the escape sequence* to detach, i.e.: press `CTRL+P` 
followed by `CTRL+Q`. Your application will continue to run and you 
can attach back anytime with the `attach` command, e.g.:

Run the following to list all active containers:

    # Usage: docker ps
    # Example:
    docker ps
    
    # CONTAINER ID       IMAGE         COMMAND           CREATED
    # 1b6a1c00d481    ubuntu:12.04    /bin/bash    About a minute ago 

And use the ID of your container to attach back:

    # Usage: docker attach [id]
    # Example:
    docker attach 1b6a1c00d481

### Setting Up A Container To Deploy Databases
----------------------------------------------------------------------

> **Note:** Once you are attached to a container, all the commands  
> executed and actions perform affect only the container and its file 
> system, without having any impact on the host -- just like a VM.
> So, go ahead and feel free to break things, *like never before*.

Once you have your container running, and yourself attached, it is time 
to get started with building it, command-by-command. Our first goal is 
to add the relevant application repositories.

**Adding general application repositories:**

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

> **Note:** For Ubuntu (or Debian), you might *not* need to add anything.
> Execute `cat /etc/apt/sources.list` to find out if relevant repository
> links already exist (and available).

Run the following to append the `universe` repository to `sources.list`:

    echo "deb http://archive.ubuntu.com/ubuntu/ precise main universe" >> \
    /etc/apt/sources.list

|| RHEL / CentOS

For RHEL / CentOS, it will be handy to have `EPEL` repository enabled.

Execute the following for `EPEL`:

    su -c 'rpm -Uvh http://dl.fedoraproject.org/pub/epel/6/x86_64/epel-release-6-8.noarch.rpm'

|

**Adding vendor-specific application repositories:**

To download and install your desired database sever, you might need to:

 - Add the vendor's application repository; or,
 - Download the source and compile the code.

For this Docker example, we will demonstrate a few of the popular 
DBMSs.

| PostgreSQL | MySQL | MongoDB | Redis | Other DBMSs

|| PostgreSQL

**Debian / Ubuntu**

Add the keys:

    apt-key adv --keyserver keyserver.ubuntu.com --recv-keys \
                B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8

Add the repository links:

    echo \
    "deb http://apt.postgresql.org/pub/repos/apt/ precise-pgdg main" \
    > /etc/apt/sources.list.d/pgdg.list

**RHEL / CentOS**

Install the PostgreSQL repository RPM for CentOS 6:
    
    # Usage: yum install -y [repository link]
    # Example:
    yum install -y http://yum.postgresql.org/9.3/redhat/rhel-6-x86_64/pgdg-centos93-9.3-1.noarch.rpm
    
Or for CentOS 5:

    yum install -y http://yum.postgresql.org/9.3/redhat/rhel-5-x86_64/pgdg-centos93-9.3-1.noarch.rpm

**Note:** Links given above are for 64 bit systems. To find the right 
package for your RHEL based distribution, visit [PostgreSQL Repository 
Packages Listing](http://yum.postgresql.org/repopackages.php) and 
install by executing the command `yum install`. Usually, it is common 
to see vendor's recommending 64-bit systems to run database servers 
for a lot of reasons that actually make sense. 

|| MySQL

MySQL does not require you to add other repository links. We can use 
the default package managers to install the application.

|| MongoDB

**Debian / Ubuntu**

Add the keys:

    apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv \
                7F0CEB10

Add the repository links:

    echo \
    'deb http://downloads-distro.mongodb.org/repo/ubuntu-upstart dist 10gen' | \    
    sudo tee /etc/apt/sources.list.d/mongodb.list                

**RHEL / CentOS**

Add the repository information for YUM:

    cat <<EOF >> /etc/yum.repos.d/mongodb.repo
    [mongodb]
    name=MongoDB Repository
    baseurl=http://downloads-distro.mongodb.org/repo/redhat/os/x86_64/
    gpgcheck=0
    enabled=1
    EOF

|| Redis

Redis does not require you to add repositories. However, we will need 
to download and install the application from source.

|| Other DBMSs

For all other databases, please visit vendor's web-page for specific 
installation instructions.

|

**Update the application repository indexes:**

Once you add or update the repository sources list, you will need to 
update the indexes for changes to come into effect, i.e.: for your 
installation search to return the desired results.

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

Update the `apt-get` package list (index):

    apt-get update

|| RHEL / CentOS

Update the package list (index):

    yum update

|

**Downloading basic essential tools:**

The next step is getting some of the basic and essential tools we are 
going to need, such as `nano` (or `vim`) for text editing, `wget` for 
certain downloads, or `dialog`. In this step, you need to make sure to 
have all libraries and tools your application needs installed. 

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

Execute the following command to get some common deployment tools:

    # Example:

    apt-get install -y aptitude
    apt-get install -y git mercurial
    apt-get install -y tar curl nano wget dialog
    apt-get install -y libevent-dev build-essential
    
|| RHEL / CentOS

Execute the following command to get some common deployment tools:

    # Example:

    yum install -y git mercurial
    yum install -y nano wget dialog curl-devel
    yum install -y which libevent-devel
    yum groupinstall -y 'development tools'

|

**Warning:** It is impossibly complicated and not actually useful 
to try to cover detailed database deployment instructions for multiple 
Linux distributions *and* multiple database servers in a single page 
document. Please take everything with a pinch-of-salt and consult a 
full installation tutorial for your specific choice of database and 
apply the missing pieces if you run into errors, or, if something's 
missing. This article's purpose is to demonstrate, by providing you 
an overall idea for deploying databases using Docker.

### Installing Databases (PostgreSQL, MySQL, MongoDB etc.)
----------------------------------------------------------------------

Once we have the necessary tools we need, and application repositories 
enabled, it is time to continue with downloading and configuring the 
database management system we would like to have *dockerised*.

Below are some generic installation instructions for a few database 
management systems (i.e. servers):

| PostgreSQL | MySQL | MongoDB | Redis | Other DBMSs

|| PostgreSQL

**Debian / Ubuntu**

To install PostgreSQL on your Debian based container, run the following:

    apt-get -y -q install postgresql-9.3 postgresql-client-9.3 postgresql-contrib-9.3

**RHEL / CentOS**

To install PostgreSQL on your CentOS based container, run the following:

    yum install -y postgresql93 postgresql93-server postgresql93-contrib    

|| MySQL

**Debian / Ubuntu**

To install MySQL on your Debian based container, run the following:

    apt-get install -y -q mysql-server

**RHEL / CentOS**

To install MySQL on your CentOS based container, run the following:

    yum install -y mysql-server mysql-devel

|| MongoDB

**Debian / Ubuntu**

To install MongoDB on your Debian based container, run the following:

    apt-get install -y -q mongodb-10gen

**RHEL / CentOS**

To install MongoDB on your CentOS based container, run the following:

    yum install -y mongo-10gen mongo-10gen-server

|| Redis

As recommended, run the following to install Redis:

    wget http://download.redis.io/redis-stable.tar.gz
    tar xvzf redis-stable.tar.gz
    cd redis-stable
    make    
    make install
    
And continue with configurations.

Or you can use the default package manager to do it for you:

    apt-get install -y -q redis-server

|| Other DBMSs

For other DBMSs, please follow the installation instructions which 
you should be able to find on respective vendor's web-site.

|

### Saving Your Database Container As A Docker Image
----------------------------------------------------------------------

Having finished building your container with the database application 
ready, you will want to save your progress (i.e. `commit`) before 
launching (i.e. `run`) a container with the application inside. 

Committing containers is a very popular and one of the most relied upon 
features of Docker. This permits you to save all your progress and use 
the *committed* image to create any number of containers.

When running (or starting) a container from an image, you will be able 
to specify certain configuration options, such as location(s) in the file 
system for the container to use. This will enable you to have the data, 
used by the database, persisted on disk *permanently*.

At its current state:

To commit your container, first exit it using the escape sequence, i.e.:

    Press CTRL+P followed by CTRL+Q

Once your are back on the host machine's prompt, run the following:

    # Usage: docker commit [options] [container ID / name] [image name]
    # Example:
    docker commit 1b6a1c00d481 redis_img

**Note:** To learn more about the `commit` command, consider reading the 
documentation on the subject: [Committing a Container]
(http://docs.docker.io/en/latest/use/workingwithrepository)

### Running Container(s) With Your Database Installation
----------------------------------------------------------------------

After having created a new Docker image with all your configurations, 
you can run the database process on any number of host computers - 
including *virtual private servers* - with Docker installed -- in any 
number of copies.

To do this, we will go back to beginning and use the `run` command. 
However, this time, we will replace `/bin/bash` with the process name 
of the database, e.g.:

| Databases With Persistent Storage | Others

|| Databases With Persistent Storage

In the below example, we are running a PostgreSQL database instance, 
from a build-and-saved Docker image called *postgresql_img* by mounting 
`/var/psqldata` directory (which needs to be created on the host) as 
`/data` for PostgreSQL process inside the container to use. This allows 
persisting the data on the host machine and not inside the container.
Once configured, PostgreSQL will start by using the `postgresql.conf` 
file found at `/var/psqldata`.

    # Usage: docker run [options] [image name] [app server run command]
    # Example:
    docker run -d -v /var/psqldata:/data \
               -p 5432:5432 \
               postgresql_img \
               --command "/usr/lib/postgresql/9.3/bin/postgres \
                          -D /data/main \
                          -c config_file=/data/postgresql.conf"

|| Others

    # Usage: docker run [options] [image name] [app server run command]
    # Example:
    docker run -d -p 6379:6379 redis_img /usr/bin/redis-server

|

**Note:** In the above example, the created container will run in the  
background, like a daemon process. The reason for this is because we 
use the `-d` flag. When you create and run a container this way, you 
will not be immediately attached to it, but you could, at a later 
time, do so.

## Automating The Build Process Using A Dockerfile
----------------------------------------------------------------------

As with pretty much anything else, manual human intervention can give 
way to faults, errors and thus, failures. Although it is quite fun to 
use a Docker container like a VM and work with it, for the greater good 
of your workflow, you are likely to want to automate things.

And container build process automation is done by using a *Dockerfile*.

Dockerfiles provide a way to script all the commands you would normally 
execute by yourself to create images and containers. Thanks to eleven 
comprehensive commands at your disposal, the entire procedure can be 
kept in a single, shareable and easy-to-use file.

Let's now see how to create a Dockerfile to automate the above explained 
procedure for containerising web application deployments.

**Note:** Below section is kept brief. The main goal here is to show 
how to craft a Dockerfile to automate the process we have been through 
manually. To learn about the Dockerfile, consider reading our dedicated 
**Introduction to Dockerfile** article. There you can see about file's 
format, instructions, syntax and how things work under-the-hood.

### Understanding The Dockerfile
----------------------------------------------------------------------

Dockerfiles are regular system files. In fact, they are no more than 
simple, plain-text documents. They come with a very simple and easy to 
author format -- and a few rules to follow, e.g.:

    # Commented out text
    [instruction] [arguments / commands]

Each Dockerfile should begin - very importantly - with a declaration of 
the base image to be used. This can be a lean Linux distribution name, 
or, someone's heavily customised application container image, e.g.:

    # Base image Ubuntu
    FROM ubuntu

Or,

    # Base image [user]/[custom image]
    FROM tutum/lamp
    
And it is customary to declare on the file the creator (i.e maintainer) 
together with a short explanation of its purpose, e.g.:

    ##################################################
    # Dockerfile sample for:
    # Docker Web Application Deployment Example
    ##################################################

    FROM ubuntu
    MAINTAINER O.S. Tezer

    # ..

After declaring the base image and file's maintainer, the commands that 
will shape the final image can be listed.

Available Dockerfile instructions (i.e. commands) are:

 - **ADD:**  
 Used for copying a file from the computer inside the container.  
 *e.g.:* Application source code.
 - **CMD:**  
 Sets the commands to be passed and/or application to be executed upon 
 container creation.  
 *e.g.:* Flags, arguments etc. passed to your web application server.
 - **ENTRYPOINT:**  
 Sets the initial - and default - application that will receive the 
 initial / default execution commands.  
 *e.g.:* Your web application server.
 - **ENV:**  
 Sets the environment variables for the container.
 - **EXPOSE:**  
 Exposes port(s) to the outside world.  
 *e.g.:* Port `8080` to allow access to the app server inside the container.
 - **FROM:**  
 Sets the base image on top of which all commands are executed to form 
 a new one.
 - **MAINTAINER:**  
 Defines Dockerfile's maintainer (i.e. creator, responsible).
 - **USER:**  
 Sets the username, or, UID that is used to execute commands during 
 container build process that are set with the `RUN` command.
 - **RUN:**  
 Used for executing commands inside a container  
 *e.g.:* Installing an application using `apt-get`.
 - **VOLUME:**  
 Allows access to a directory from the container to the host machine.
 - **WORKDIR:**  
 Sets the base directory where process execution command is to run.  
 *e.g.:* Base directory for executing your web application server.

> **Tip:** Dockerfiles are flexible and allow you to achieve a great deal of things through a collection of directives / commands. To learn about the Dockerfile auto-builder, check out the documentation page [Dockerfile Reference](http://docs.docker.io/en/latest/reference/builder).

### Crating A Dockerfile To Automate Web Application Deployments
----------------------------------------------------------------------

Dockerfiles are regular system files. In fact, they are no more than 
simple, plain-text documents. They come with a very simple and easy to 
author format -- and a few rules to follow.

Create a new text file that is called `Dockerfile`, e.g.:

    nano Dockerfile

And list all the commands to be executed successively after defining 
the base image and the file's maintainer, e.g.:

    #
    # example Dockerfile for http://docs.docker.io/en/latest/examples/postgresql_service/
    #
    
    FROM ubuntu
    MAINTAINER SvenDowideit@docker.com
    
    # Add the PostgreSQL PGP key to verify their Debian packages.
    # It should be the same key as https://www.postgresql.org/media/keys/ACCC4CF8.asc 
    RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8
    
    # Add PostgreSQL's repository. It contains the most recent stable release
    #     of PostgreSQL, ``9.3``.
    RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ precise-pgdg main" > /etc/apt/sources.list.d/pgdg.list
    
    # Update the Ubuntu and PostgreSQL repository indexes
    RUN apt-get update
    
    # Install ``python-software-properties``, ``software-properties-common`` and PostgreSQL 9.3
    #  There are some warnings (in red) that show up during the build. You can hide
    #  them by prefixing each apt-get statement with DEBIAN_FRONTEND=noninteractive
    RUN apt-get -y -q install python-software-properties software-properties-common
    RUN apt-get -y -q install postgresql-9.3 postgresql-client-9.3 postgresql-contrib-9.3
    
    # Note: The official Debian and Ubuntu images automatically ``apt-get clean``
    # after each ``apt-get`` 
    
    # Run the rest of the commands as the ``postgres`` user created by the ``postgres-9.3`` package when it was ``apt-get installed``
    USER postgres
    
    # Create a PostgreSQL role named ``docker`` with ``docker`` as the password and
    # then create a database `docker` owned by the ``docker`` role.
    # Note: here we use ``&&\`` to run commands one after the other - the ``\``
    #       allows the RUN command to span multiple lines.
    RUN    /etc/init.d/postgresql start &&\
        psql --command "CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" &&\
        createdb -O docker docker
    
    # Adjust PostgreSQL configuration so that remote connections to the
    # database are possible. 
    RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/9.3/main/pg_hba.conf
    
    # And add ``listen_addresses`` to ``/etc/postgresql/9.3/main/postgresql.conf``
    RUN echo "listen_addresses='*'" >> /etc/postgresql/9.3/main/postgresql.conf
    
    # Expose the PostgreSQL port
    EXPOSE 5432
    
    # Add VOLUMEs to allow backup of config, logs and databases
    VOLUME	["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]
    
    # Set the default command to run when starting the container
    CMD ["/usr/lib/postgresql/9.3/bin/postgres", "-D", "/var/lib/postgresql/9.3/main", "-c", "config_file=/etc/postgresql/9.3/main/postgresql.conf"]

And save the file.

### Using A Dockerfile To Build Images And Run Containers
----------------------------------------------------------------------

Once our Dockerfile is ready, we can use `docker build` to build a new 
container image successively, instruction-by-instruction.

Run the following command to build a new image:

    # Usage: docker build -t [image name] .
    # Example:
    docker build -t eg_postgresql .

As the console output will show, `docker` will execute all instructions 
and provide you with a brand new image which you can use to instantiate 
Docker containers, e.g.:

    docker run -rm -P -name pg_test \
                            eg_postgresql

You can now enjoy your brand new, highly portable, secure and isolated 
isolated container that is running your web application.

**Tip:** The -rm removes the container and its image when the container 
exists successfully.

## Questions & Ideas Which Might Pop Up In Your Mind
----------------------------------------------------------------------

Please contact us with your questions and suggestions for us to make 
this article better.

Submitted by: [O.S. Tezer](https://twitter.com/ostezer)
