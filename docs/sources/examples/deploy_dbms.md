# Deploying Databases Using Docker
----------------------------------------------------------------------

## Introduction And Contents
----------------------------------------------------------------------

### Introduction
----------------------------------------------------------------------

Deployment and management of databases is a significant challenge with 
a lot of tricky parts involved. Handling this in a correct way means 
preventing data loss, implementing failover mechanisms, ensuring security 
and more.

In this example, we will see how to use Docker to simplify this process 
by using *containers*. Through this tutorial we will also learn the best 
and most common practices of running database applications inside them.

**By the end of this example, you will know how to:**

 - Deploy and manage databases using containers.
 - Separate the data persistence layer from *inside* to the *outside*
 of containers.
 - Automate the above mentioned process using with *Dockerfiles*.

## Understanding The Challenge
----------------------------------------------------------------------

### Application Deployment
----------------------------------------------------------------------

The word *deployment* is generally used when talking about the series 
of actions performed to make a higher-level application accessible to 
the intended user(s) and other clients alike.

In a very broad sense it signifies something being put into service, 
*ready to be operated*.

Although the details of this is very much open to discussion and the 
term itself is rather subjective, for almost everyone, it means 
*getting up-and-running*. 

### Deploying And Managing Databases
----------------------------------------------------------------------

Depending entirely on a single type of database is not enough for many
applications. Given today's technologies and trends, it wouldn't be 
uncommon to think of a scenario whereby a couple NoSQL databases, 
together with a relational database server set-up being in use.

For deployments, all this translates to preparing different sever(s) 
with custom configuration options for each. When coupled with challenges 
of management, the curve of dealing with all this can get really steep.

With its *objective* approach towards application hosting, Docker 
containers can help you with high-availability, implementing fail-over mechanisms in an easier and more logical way, and more.

## Docker For Relational Database Management Systems
----------------------------------------------------------------------

Relational database management systems are the go-to solution for many 
applications thanks to their *functionality* and *dependability*.

For advanced systems such as *MySQL* or *PostgreSQL*, keeping the data 
permanently without explicitly disposing is a must. The ability to share 
volumes (i.e. parts of the storage on the host machine) makes it possible 
to back-up, upgrade, and cluster (multiple) database servers easily 
compared to working with basic servers or server images.

### Default Container Data Storage
----------------------------------------------------------------------

Docker containers are designed to be lightweight and easy to handle for 
the host machine.

When you create (or start, i.e. `docker run`) a process inside a Docker   container, it doesn't have outside access. Unless explicitly specified, 
all the data kept inside is disposed when the container stops -- leaving 
the base image as it was without any changes.

Put simply, all the data that gets downloaded and installed, together 
with outputs and other information the running process generates are 
temporary. Only containers that are committed keep the data after 
being stopped -- which translates to creating a new image with a larger 
footprint.

### Persistent Data Storage With Containers
----------------------------------------------------------------------

Certain applications, including and especially databases, require the 
data to be persistent. Meaning: no matter what happens to the process,
the data should stay on the host and it should be accessible. This 
way of working with containers also ensures that containers are kept 
lean, the application logic separated and with portability still
intact.

In order to achieve this persistence, *volumes* are used with Docker.

> **Note:** To learn more about volumes, consider checkout out the 
> documentation on the subject: [Share Directories via Volumes](
> http://docs.docker.io/en/latest/use/working_with_volumes/).

For applications such as MySQL whereby version of the database itself 
defines the format of the data, a simple host mount or volume creation 
during build can be sufficient. In fact, it will offer more-than-enough 
flexibility for your deployments.

However, for solutions such as PostgreSQL, using another container 
like a proxy for storage is the better way for deploying. For various 
reasons, including  security and portability, databases should work  
together with a *storage container* by making use of the `volumes from` argument.

This method helps to extract complexities of dealing with storage,
since your container becomes independent of the data and objectifies 
the whole process where database application is separated from data 
files.

In this example, we will go through both.

## Understanding The Workflow
----------------------------------------------------------------------

In order to extract data storage from within the containers, we will 
use volumes. There are two ways of doing this, each with their own 
benefits.

With database systems such as MySQL, you can simply opt for volumes 
directly.

For systems like PostgreSQL where you would like to have multiple 
thin database processes accessing the data, *Data Volume Containers* 
are the better option.

Therefore, the workflow consists of the following:

 - **Creating a Data Volume Container:**  
 Data Volume Containers (DVCs) allow you to access persistent data  consistently -- and share them between other containerised processes.
 - **Creating a Database Container:**  
 Database [hosting] containers will use the *volumes from* the DVC 
 and work in a portable, lightweight and predictable way.

### Steps to Create The Data Volume Containers
----------------------------------------------------------------------

Data Volume Containers are absolutely basic Docker containers that 
are created with volumes. Their sole purpose is to offer the data 
persistence platform for other containers that need consistent access.

> **Note:** DVCs do not need to be running to perform their duty.

Creating a DVC consist of the following step:

1. **Create a basic container with mounted volumes:**  
   By either using a Dockerfile, or manually, start a new, preferably 
   tagged container with volumes attached.

### Steps to Create The Application Containers
----------------------------------------------------------------------

Building a container usually consists of the following steps:

1. **Adding the repository links:**  
   Default base images are usually kept lean. Therefore, we need to 
   append the application repository links we need access to, in order 
   to download the database management system we want to use.
2. **Updating the default package manager sources:**  
   Once new repository links are added (e.g. Ubuntu's `universe` or 
   `EPEL` for RHEL-based distributions, or custom vendor repositories), 
   we need to update the package indexes to have access to applications.
3. **Downloading the basic tools needed to work with the containers:**  
   Containers are absolutely isolated. In case of using a simple base 
   image, even for basic tasks, you will need some tools installed. 
   This means getting applications like `wget`, `nano`, or essential 
   build software (e.g. `make`) -- just like the older days of CentOS.
4. **Installing the main process (e.g. the database server):**  
   After getting all the tools, the next step will be installing the 
   database server itself.
5. **Configuring the database:**  
   Next, as expected, we will need to configure the database.
6. **Running the database:**  
   Once the application is installed and configured, it is time to run 
   the database by specifying the DVC's name to persist the data.

Whether working with containers manually or building them automatically 
using Dockerfiles, most of the process explained above will remains the 
same.

Let's begin!

## Creating A Data Volume Container
----------------------------------------------------------------------

### By Starting A New Container
----------------------------------------------------------------------

You can simply create a DVC with the `docker run` command and passing 
volumes as arguments using the `-v` flag, i.e.:

    # Usage: docker run [-v /path/to/volume] [name] [image] [process]
    # Example:
    docker run -v /etc/postgresql \
               -v /var/log/postgresql \
               -v /var/lib/postgresql \
               -name PSQL_DATA \
               busybox true

### By Using A Dockerfile
----------------------------------------------------------------------

In order to automate the process in a maintainable way, you can choose 
working with a Dockerfile instead:

    ####################################################
    # Dockerfile to manage data volumes for PostgreSQL #
    ####################################################
    
    FROM          busybox
    MAINTAINER    O.S. Tezer, ostezer@gmail.com
     
    VOLUME        ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]
    CMD           ["/bin/true"]


Once the Dockerfile is ready, you can build an image by executing:
    
    docker build -t psql_data .

And then you can run (or initiate) the container with:

    docker run -name PSQL_DATA psql_data

And that's it! Our DVC is ready.

## Creating A Database Application Container
----------------------------------------------------------------------

Dockerfiles provide a way to script all the commands you would normally 
execute by yourself to create images and containers. Thanks to eleven 
comprehensive commands at your disposal the entire procedure can be 
kept in a single file. Since they are highly maintainable and easier to 
manage, it is much more preferable to build a container using them.

Below is an example to prepare a container from base `ubuntu` image 
to run PostgreSQL instances:

    #########################################################
    # Dockerfile to host light-weight PostgreSQL instances  #
    # using Data Volume Containers as persistence objects   #
    #########################################################
    
    FROM ubuntu
    MAINTAINER Sven Dowideit, SvenDowideit@docker.com
    
    # Add the PostgreSQL PGP key to verify their Debian packages.
    # It should be the same key as:
    # https://www.postgresql.org/media/keys/ACCC4CF8.asc 
    
    RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8
    
    # Add PostgreSQL's repository.
    # It contains the most recent stable release
    # of PostgreSQL 9.3
    RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ precise-pgdg main" > /etc/apt/sources.list.d/pgdg.list
    
    # Update the Ubuntu and PostgreSQL repository indexes
    RUN apt-get update
    
    # Install:
    # python-software-properties,
    # software-properties-common,
    # PostgreSQL 9.3
    # There are some warnings (in red) that show up during the build.
    # You can hide them by prefixing each apt-get statement with:
    # DEBIAN_FRONTEND=noninteractive
    RUN apt-get -y -q install python-software-properties software-properties-common
    RUN apt-get -y -q install postgresql-9.3 postgresql-client-9.3 postgresql-contrib-9.3
    # Note: The official Debian and Ubuntu images automatically
    # apt-get clean after each apt-get 
    
    # Run the rest of the commands as the postgres user created by
    # the postgres-9.3 package when it was apt-get installed
    USER postgres
    
    # Create a PostgreSQL role named 
    # docker
    # with
    # docker
    # as the password and then;
    # create a database docker owned by the docker role.
    # Note: here we use &&\ to run commands one after the other
    # the \ allows the RUN command to span multiple lines.
    RUN /etc/init.d/postgresql start && \
        psql --command "CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" && \
        createdb -O docker docker
    
    # Adjust PostgreSQL configuration so that remote connections to the
    # database are possible. 
    RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/9.3/main/pg_hba.conf
    
    # And add:
    # listen_addresses to:
    # /etc/postgresql/9.3/main/postgresql.conf
    RUN echo "listen_addresses='*'" >> /etc/postgresql/9.3/main/postgresql.conf
    
    # Expose the PostgreSQL port:
    EXPOSE 5432
        
    # Set the default command to run when starting the container
    CMD ["/usr/lib/postgresql/9.3/bin/postgres", "-D", "/var/lib/postgresql/9.3/main", "-c", "config_file=/etc/postgresql/9.3/main/postgresql.conf"]

And save the file.

> **Tip:** Docker skips unsupported instructions with a warning.

Once our Dockerfile is ready, we can use `docker build` to build a new 
container image successively, instruction-by-instruction. Each command 
executed during the build creates a new image, and the next one uses 
that [new] image to execute the next, forming the layers of the Docker 
onion.

As the console output will show, `docker` will execute all instructions 
and provide you with a brand new image which you can use to instantiate 
Docker containers, e.g.:

Run the following command to build a new image:

    # Usage: docker build -t [image name] .
    # Example:
    docker build -t psql_93_image .

## Using A Dockerfile Run Database Containers
----------------------------------------------------------------------

### Without DVCs
----------------------------------------------------------------------

Run the following to start a new database container using mounted volumes:

    docker run --rm -P \
               -v /etc/postgresql \
               -v /var/log/postgresql \
               -v /var/lib/postgresql \
               -name pg_test \
               psql_93_image


### With DVCs
----------------------------------------------------------------------

Run the following to start a new database container using a DVC:

    docker run --rm -P \
               -volumes-from psql_data \
               -name pg_test \
               psql_93_image

> **Tip:** The `--rm` removes the container when it exists successfully.

> **Tip:** The `-P` flag exposes all the ports specified in the 
> Dockerfile with random public ports attached.

Submitted by: [O.S. Tezer](https://twitter.com/ostezer)
