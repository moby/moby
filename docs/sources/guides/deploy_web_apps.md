# How To Deploy Web Applications Using Docker
----------------------------------------------------------------------

## Introduction And Contents
----------------------------------------------------------------------

### Introduction
----------------------------------------------------------------------
Web applications come in many different flavours -- usually mixed and
matched with various other elements, such as databases. Distinct from
the programming language used, or the application size, the deployment
process can be pretty straight forward with the right guidance. When
the entire *application lifecycle* (ALM) considered, however, the whole 
thing can easily turn into bit of a mess.

In this example, we are going to see how to use Docker as a powerful 
base for deploying web applications. We will do this not by remodelling 
or introducing a drastic change, but rather, rethinking and using 
containers  as reliable, platform-agnostic and portable building-blocks 
to simplify *everything*.

> **Note:** Before we take a look now to see how to make everybody happy 
> (including and especially the *system-operators*), please remember: 
> you need to have your web-application repository ready and the choice  
> of app server you'd love to use made. Docker doesn't replace much of 
> anything, other than ye olde undeserved frustration.

### Contents
----------------------------------------------------------------------

1. Understanding The Challenge
    1. Application Deployment
    2. Web Application Deployment
    3. Application Lifecycle Management (ALM)
2. Introducing Docker Into The Challenge
3. Building A Docker Container Step-by-Step
    1. Understanding The Workflow
    2. Instantiating A Container
    3. Setting Up A Container For Web Applications
    4. Installing Interpreters (Python, Ruby)
    5. Installing Web Application Servers
    6. Getting Your Web Application Code Inside The Container
    7. Running Your Web Application
    8. Saving Your Web Application Container To An Image
    9. Running Container(s) With Your Web Application From A Saved Image
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

### Web Application Deployment
----------------------------------------------------------------------

For web applications, regardless of size or technology used, deployment 
can convey two things: a huge milestone for going into production, or, 
a very mundane task that has to be repeated with each update.

In summary, the process consists of setting everything up, specially 
prepared for the target platform and then finally uploading the code and 
running it all.

Even the basics of this can be problematic, however, due to differences 
between development, testing, production and management (ALM) - which 
can all be taken care of, efficiently, by incorporating *Docker* in the 
mix.

### Application Lifecycle Management (ALM)
----------------------------------------------------------------------

Going online, despite holding an enormous value, is only a single aspect 
of continuous creation. As mentioned before, application lifecycle 
management consists of various other steps -- and in reality, rarely do 
they take place on the same system, or even architecture. 

By levelling the differences using containers, a lot of headaches can be 
waved goodbye. From a single page "Hello world!" website to projects 
welcoming hundreds of millions of clients, standardising *your* ALM will 
translate to leverage: reduced times, lowered costs, simpler integration 
and *much more*.

## Introducing Docker Into The Challenge
----------------------------------------------------------------------

Docker, by nature, is agnostic. It is a simple, but extremely powerful 
set of tools which can be used almost anywhere, with anything. It does 
not aim to introduce changes but rather eliminate parts that cause 
trouble.

Docker helps you to achieve this by *containerisation*.

As explained in the Getting-Started articles, containers offer a way of 
keeping everything related to a project in one place, on top of a base 
operating-system disk image. By taking advantage of this technology, 
web application deployment can be simplified in the following ways:

1. **Simple automation:**  
   Docker containers can be built automatically to your application's 
   specification.
2. **Simple deployments:**  
   Containers are highly portable. They only require system operators
   (or single developers) to push them to their servers to get up and 
   running -- or scale across any number of machines.
3. **Easy scaling:**  
   Using containers, duplicating instances running your application 
   can be increased (or decreased) by executing a single command for 
   each operation respectively.
4. **Enhanced security:**  
   Containers, and applications deployed inside, can be considered 
   like powerful sandboxes. They do not have access to the outside,
   and the affects of any command executed stays within.

Continuing in this example, we will now see how to treat a container 
like a virtual machine, connect to it, prepare it like a computer of 
its own, and then commit the end result to be used in the future -- 
to deploy / launch instances running web applications.

Next, we will see how to craft a *Dockerfile* script to automate image 
creation, or the *build process*. This way, we are going to have a 
Docker container that will be built, step-by-step, with your application 
repository copied and set automatically inside, ready to work. 

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
Afterwards, we need to build it to run our web application. In general, 
this process will consist of the following steps:

1. **Add repository links:**  
   Default base images are usually kept lean. Therefore, we need to 
   append the application repository links we need access to, in order 
   to download the tools for the job, e.g. Unicorn, Gunicorn, Nginx etc.
2. **Update default package manager sources:**  
   Once new repository links are added (e.g. Ubuntu's `universe` or 
   `EPEL` for RHEL-based distributions), we need to update the packages.
3. **Download basic tools we will need:**  
   Containers are absolutely isolated. In case of using a simple base 
   image, even for basic tasks, you will need to have tools installed.
   This means getting applications like `wget`, `nano`, or essential 
   build software (e.g. `make`) -- just like the older days of CentOS.
4. **Install and configure applications for deployment:**  
   Deploying and running a web-application requires using a web server,
   and (usually) a source code interpreter (e.g. Python, Ruby). Before 
   getting the application code, we need to have the server configured 
   and ready.
5. **Getting your web application:**  
   Once the container is ready to run your web application, it is time 
   to get your code onto the system (i.e. inside the container).
6. **Running your web application:**  
   After everything is set, it is time to go into production -- at 
   scale!

### Instantiating A Container
----------------------------------------------------------------------

In order to work with Docker containers, the first thing to do is to 
create one. Container creation process consists of executing the `run` 
command (i.e. `docker run`) and specifying the name of a `base image` to 
use. Afterwards, we need to provide `docker` with options and settings, 
together with the application we would like to have it running. This is 
necessary for `docker` to instantiate the container.

**Note:** `docker run` command is not platform specific. We will use 
the below examples, however, to run containers with different Linux 
distributions.

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

In this example, we will `expose` port `8080`. This is going to allow 
access from the outside world to the web application we will have 
running inside the container.

Execute the following command to have a container running `bash`:

    docker run -i -t -p 8080:8080 ubuntu /bin/bash

|| RHEL / CentOS

In this example, we will `expose` port `8080`. This is going to allow 
access from the outside world to the web application we will have 
running inside the container.

Execute the following command to have a container running `bash`:

    docker run -i -t -p 8080:8080 centos /bin/bash

|

> If the given base image is not found on the host machine where the 
`docker` daemon is running, it gets downloaded for use.

**Naming (i.e. tagging) a container:**

If you would like to name (i.e. `tag`) a container, you can use the 
`-name` parameter, e.g.:

    # Usage: docker run -name [name] [.. options] [image] [process]
    # Example:
    docker run -name my_container -i -t -p 8080:8080 centos /bin/bash

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

### Setting Up A Container For Web Applications
----------------------------------------------------------------------

> **Note:** Once you are attached to a container, all the commands  
> executed and actions perform affect only the container and its file 
> system, without having any impact on the host -- just like a VM.
> So, go ahead and feel free to break things, *like never before*.

Once you have your container running, and yourself attached, it is time 
to get started with building it, command-by-command. Our first goal is 
to add the relevant application repositories.

**Adding and updating application repositories:**

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

> **Note:** For Ubuntu (or Debian), you might *not* need to add anything.
> Execute `cat /etc/apt/sources.list` to find out if relevant repository
> links already exist (and available).

Run the following to append the `universe` repository to `sources.list`:

    echo "deb http://archive.ubuntu.com/ubuntu/ precise main universe" >> \
    /etc/apt/sources.list

Update the `apt-get` package list:

    apt-get update

|| RHEL / CentOS

For RHEL / CentOS, it will be handy to have `EPEL` repository enabled.

Execute the following for `EPEL`:

    su -c 'rpm -Uvh http://dl.fedoraproject.org/pub/epel/6/x86_64/epel-release-6-8.noarch.rpm'
    
And update:

    yum update

|

**Downloading basic essential tools:**

The next step is getting some of the basic and essential tools we are 
going to need, such as `nano` (or `vim`) for text editing, `wget` for 
certain downloads or `dialog`. In this step, you need to make sure to 
have all libraries and tools your application needs installed. 

| Debian / Ubuntu | RHEL / CentOS

|| Debian / Ubuntu

Execute the following command to get the tools you use for deploying 
your web application:

    # Example:

    apt-get install -y aptitude
    apt-get install -y git mercurial
    apt-get install -y tar curl nano wget dialog
    apt-get install -y libevent-dev build-essential
    
|| RHEL / CentOS

Execute the following command to get the tools you use for deploying 
your web application:

    # Example:

    yum install -y git mercurial
    yum install -y nano wget dialog curl-devel
    yum install -y which libevent-devel
    yum groupinstall -y 'development tools'

|

### Installing Interpreters (Python, Ruby)
----------------------------------------------------------------------

Once we have the necessary tools we need, and application repositories 
enabled, it is time to continue with setting up our system for web 
application deployments. For this, we will need to get the respective 
code executing interpreters (i.e. Python, Ruby etc.).

> **Note:** In this example, we are going to focus on Ruby and Python. 
> Please remember that the same principal applies for other languages 
> and their respective interpreters.

** Debian / Ubuntu **

| Python | Ruby

|| Python

Run the following to install Python and core Python tools:

    apt-get install -y python python-dev python-pip python-distribute
 
And get the libraries you need, e.g. database drivers:

    # PostgreSQL:
    apt-get install -y python-psycopg2
    
    # MySQL:
    apt-get install -y python-mysqldb libmysqlclient-dev

|| Ruby

Run the following to install RVM and Ruby `2.1.0`:

    curl -L get.rvm.io | bash -s stable
    source /etc/profile.d/rvm.sh
    rvm reload
    rvm install 2.1.0

|

** RHEL / CentOS **

| Python | Ruby

|| Python

Run the following to install some important packages:

    yum install -y xz zlib-dev bzip2-devel
    yum install -y openssl-devel sqlite-devel 

Run the following to install Python:

    wget http://www.python.org/ftp/python/2.7.6/Python-2.7.6.tar.xz
    xz -d Python-2.7.6.tar.xz
    tar -xvf Python-2.7.6.tar
    cd Python-2.7.6
    ./configure --prefix=/usr/local
    make
    make altinstall

Append the installation path to *your* `PATH` variable:

    export PATH="/usr/local/bin:$PATH"

Install core Python tools:

    # Setuptools
    wget --no-check-certificate https://pypi.python.org/packages/source/s/setuptools/setuptools-1.4.2.tar.gz
    tar -xvf setuptools-1.4.2.tar.gz
    cd setuptools-1.4.2
    python2.7 setup.py install
    
    # pip
    curl https://raw.github.com/pypa/pip/master/contrib/get-pip.py | python2.7 -
    
    # virtualenv
    pip install virtualenv    

|| Ruby

    yum install -y zlib-dev bzip2-devel openssl-devel sqlite-devel
    ln -sf /proc/self/fd /dev/fd
    curl -L get.rvm.io | bash -s stable
    source /etc/profile.d/rvm.sh
    rvm reload
    rvm install 2.1.0

|

### Installing Web Application Servers
----------------------------------------------------------------------

As the next step before getting the web application itself, we need to 
download and install an application server to run your web app. Since 
there is a multitude of choices available, we are going to go with a 
generic and common choice for each programming language we mentioned.

| Python | Ruby

|| Python

For Python, our example choice is Gunicorn.

Execute the following command to install Gunicorn using `pip`:

    pip install gunicorn

|| Ruby

For Ruby, our example choice is Unicorn.

Execute the following command to install Unicorn using `pip`:

    gem install unicorn

|

### Getting Your Web Application Code Inside The Container
----------------------------------------------------------------------

After having the container set-up, much like working with a brand new 
computer, we can get our web-application repository and start the web 
server.

For this purpose, you can use any tool -- `pip`, `gem`, `git` or even 
`wget`.

| git | wget | pip | gem

|| git

Create a web-application deployment directory:

    mkdir /var/www

Enter the directory:

    cd /var/www

To get your application code using `git`, run the following:

    # Usage: git clone [uri]
    # Example:
    git clone https://github.com/shykes/helloflask.git

|| wget

Create a web-application deployment directory:

    mkdir /var/www

Enter the directory:

    cd /var/www
    
To get your application code using `wget`, run the following:

    # Usage: wget [uri]
    # Example:
    wget https://github.com/shykes/helloflask/tarball/master
    
|| pip

Create a web-application deployment directory:

    mkdir /var/www

Enter the directory:

    cd /var/www

To get your application code using `pip`, run the following:

    # Usage: pip install [Python application name from PyPi]
    # Example:
    pip install flask
    
|| gem

Create a web-application deployment directory:

    mkdir /var/www

Enter the directory:

    cd /var/www

To get your application code using `gem`, run the following:

    # Usage: gem install [Ruby application name from RubyGems]
    # Example:
    gem install rails --no-ri --no-rdoc
    
|

Once you get your application code, you can continue as per the usual 
to get it set up, i.e.

1. Code extracted (if needed) and ready;
2. Dependencies installed (e.g. `pip install -r requirements.txt`);
3. Web application configuration file ready (e.g. `config/unicorn.rb`). 

### Running Your Web Application
----------------------------------------------------------------------

The last and final step to run your web application inside a container 
is actually launching it.

| Python (`gunicorn`) | Ruby (`unicorn`)

|| Python (`gunicorn`)

Execute the following command to run your web application using `gunicorn`:

    # Usage: gunicorn [options] [wsgi file]
    # Example:
    gunicorn -b 0.0.0.0:8080 wsgi

|| Ruby (`unicorn`)

Execute the following command to run your web application using `unicorn`:

    # Usage: unicorn_rails
    # Example:
    unicorn_rails -c config/unicorn.rb

|

### Saving Your Web Application Container To An Image
----------------------------------------------------------------------

Having finished building your container and getting it to run your web 
application, you will want to save your progress (i.e. `commit`).

Committing containers is a very popular and one of the most relied upon 
features of Docker. This permits you to save all your progress and use 
the *committed* image to create any number of containers running your 
application.

To commit your container, first exit it using the escape sequence, i.e.:

    Press CTRL+P followed by CTRL+Q

Once your are back on the host machine's prompt, run the following:

    # Usage: docker commit [options] [container ID / name] [image name]
    # Example:
    docker commit 1b6a1c00d481 my_python_web_application_container_image

**Note:** To learn more about the `commit` command, consider reading the 
documentation on the subject: [Committing a Container]
(http://docs.docker.io/en/latest/use/workingwithrepository)

### Running Container(s) With Your Web Application From A Saved Image
----------------------------------------------------------------------

After having created a new Docker image with all your changes in place 
(i.e. installations, configurations etc.), you can run your application 
on any number of host computers - including *virtual private servers* - 
with Docker installed -- in any number of copies.

To do this, we will go back to beginning and use the `run` command.
However, this time, we will replace `/bin/bash` with your app server's 
executable path and any configuration parameters the application needs.

To start a new container running your web application, run the following:

| Python (`gunicorn`) | Ruby (`unicorn`)

|| Python (`gunicorn`)

    # Usage: docker run [options] [image name] [app server run command]
    # Example:
    docker run -d -p 8080:8080 my_python_web_application_container_image \
               gunicorn -b 0.0.0.0:8080 \
                        --pythonpath=/var/www/helloflask \
                        app:app

|| Ruby (`unicorn`)

    # Usage: docker run [options] [image name] [app server run command]
    # Example:
    docker run -d -p 8080:8080 my_python_web_application_container_image \
               unicorn_rails --path /var/www/.. \
                             -c config/unicorn.rb

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
manually. To learn about the Dockerfile, consider reading our dedicated **Introduction to Dockerfile** article. There you can see about file's 
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

    ##################################################
    # Dockerfile sample for:
    # Python Web App Deployment Example on Ubuntu
    ##################################################

    FROM ubuntu
    MAINTAINER O.S. Tezer

    # Note !
    # Please make sure the commands executed via RUN 
    # do not prompt messages or bring up interactive 
    # installation screens (i.e. dialogs). 

    # Install basic tools on Ubuntu for web app deployment:
    RUN apt-get update
    RUN apt-get install -y -q git mercurial
    RUN apt-get install -y -q tar curl nano wget
    RUN apt-get install -y -q libevent-dev build-essential
    
    # Install Python tools:
    RUN apt-get install -y -q python python-dev
    RUN apt-get install -y -q python-pip python-distribute
    
    # Install Gunicorn web application server:
    RUN pip install gunicorn
    
    # Get your web application source using Git:
    RUN git clone https://github.com/shykes/helloflask.git
    
    ## Or, by using ADD:
    # ADD /helloflask /helloflask
    
    # Install requirements:
    RUN pip install -r /helloflask/requirements.txt
    
    # Set the base directory of your application:
    WORKDIR /helloflask
    
    # Set the port to be exposed:
    EXPOSE 8080
    
    # Set the command and arguments to execute upon launch: 
    CMD gunicorn -b 0.0.0.0:8080 app:app

And save the file.

> **Tip:** Why don't you try building a CentOS based container, set to 
> run a Ruby-on-Rails application by replacing the base image to CentOS 
> and adding the relevant `RUN` commands to install Ruby and then your 
> Rails based application? It is fun!

### Using A Dockerfile To Build Images And Run Containers
----------------------------------------------------------------------

Once our Dockerfile is ready, we can use `docker build` to build a new 
container image successively, instruction-by-instruction.

Run the following command to build a new image:

    # Usage: docker build -t [image name] .
    # Example:
    docker build -t helloflask_img .

As the console output will show, `docker` will execute all instructions 
and provide you with a brand new image which you can use to instantiate 
Docker containers, e.g.:

    docker run -name helloflask_app_container \
               -d -p 8080:8080 helloflask_img

You can now enjoy your brand new, highly portable, secure and isolated 
isolated container that is running your web application.

## Questions & Ideas Which Might Pop Up In Your Mind
----------------------------------------------------------------------

Please contact us with your questions and suggestions for us to make 
this article better.

Submitted by: [O.S. Tezer](https://twitter.com/ostezer)