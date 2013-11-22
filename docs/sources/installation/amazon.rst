:title: Installation on Amazon EC2
:description: Docker installation on Amazon EC2 
:keywords: amazon ec2, virtualization, cloud, docker, documentation, installation

Amazon EC2
==========

.. include:: install_header.inc

There are several ways to install Docker on AWS EC2:

* :ref:`amazonquickstart` or
* :ref:`amazonstandard` or
* :ref:`amazonvagrant`

**You'll need an** `AWS account <http://aws.amazon.com/>`_ **first, of course.**

.. _amazonquickstart:

Amazon QuickStart
-----------------

1. **Choose an image:**

   * Launch the `Create Instance Wizard` <https://console.aws.amazon.com/ec2/v2/home?#LaunchInstanceWizard:> menu on your AWS Console
   * Select "Community AMIs" option and serch for ``amd64 precise`` (click enter to search)
   * If you choose a EBS enabled AMI you will be able to launch a `t1.micro` instance (more info on `pricing` <http://aws.amazon.com/en/ec2/pricing/> )
   * When you click select you'll be taken to the instance setup, and you're one click away from having your Ubuntu VM up and running.

2. **Tell CloudInit to install Docker:**

   * Enter ``#include https://get.docker.io`` into the instance *User
     Data*. `CloudInit <https://help.ubuntu.com/community/CloudInit>`_
     is part of the Ubuntu image you chose and it bootstraps from this
     *User Data*.

3. After a few more standard choices where defaults are probably ok, your
   AWS Ubuntu instance with Docker should be running!

**If this is your first AWS instance, you may need to set up your
Security Group to allow SSH.** By default all incoming ports to your
new instance will be blocked by the AWS Security Group, so you might
just get timeouts when you try to connect.

Installing with ``get.docker.io`` (as above) will create a service named
``lxc-docker``. It will also set up a :ref:`docker group <dockergroup>` and you
may want to add the *ubuntu* user to it so that you don't have to use ``sudo``
for every Docker command.

Once you've got Docker installed, you're ready to try it out -- head
on over to the :doc:`../use/basics` or :doc:`../examples/index` section.

.. _amazonstandard:

Standard Ubuntu Installation
----------------------------

If you want a more hands-on installation, then you can follow the
:ref:`ubuntu_linux` instructions installing Docker on any EC2 instance
running Ubuntu. Just follow Step 1 from :ref:`amazonquickstart` to
pick an image (or use one of your own) and skip the step with the
*User Data*. Then continue with the :ref:`ubuntu_linux` instructions.

.. _amazonvagrant:

Use Vagrant
-----------

.. include:: install_unofficial.inc
  
And finally, if you prefer to work through Vagrant, you can install
Docker that way too. Vagrant 1.1 or higher is required.

1. Install vagrant from http://www.vagrantup.com/ (or use your package manager)
2. Install the vagrant aws plugin

   ::

       vagrant plugin install vagrant-aws


3. Get the docker sources, this will give you the latest Vagrantfile.

   ::

      git clone https://github.com/dotcloud/docker.git


4. Check your AWS environment.

   Create a keypair specifically for EC2, give it a name and save it
   to your disk. *I usually store these in my ~/.ssh/ folder*.

   Check that your default security group has an inbound rule to
   accept SSH (port 22) connections.

5. Inform Vagrant of your settings

   Vagrant will read your access credentials from your environment, so
   we need to set them there first. Make sure you have everything on
   amazon aws setup so you can (manually) deploy a new image to EC2.

   Note that where possible these variables are the same as those honored by
   the ec2 api tools.
   ::

       export AWS_ACCESS_KEY=xxx
       export AWS_SECRET_KEY=xxx
       export AWS_KEYPAIR_NAME=xxx
       export SSH_PRIVKEY_PATH=xxx

       export BOX_NAME=xxx
       export AWS_REGION=xxx
       export AWS_AMI=xxx
       export AWS_INSTANCE_TYPE=xxx

   The required environment variables are:

   * ``AWS_ACCESS_KEY`` - The API key used to make requests to AWS
   * ``AWS_SECRET_KEY`` - The secret key to make AWS API requests
   * ``AWS_KEYPAIR_NAME`` - The name of the keypair used for this EC2 instance
   * ``SSH_PRIVKEY_PATH`` - The path to the private key for the named
     keypair, for example ``~/.ssh/docker.pem``

   There are a number of optional environment variables:

   * ``BOX_NAME`` - The name of the vagrant box to use.  Defaults to
     ``ubuntu``.
   * ``AWS_REGION`` - The aws region to spawn the vm in.  Defaults to
     ``us-east-1``.
   * ``AWS_AMI`` - The aws AMI to start with as a base.  This must be
     be an ubuntu 12.04 precise image.  You must change this value if
     ``AWS_REGION`` is set to a value other than ``us-east-1``.
     This is because AMIs are region specific.  Defaults to ``ami-69f5a900``.
   * ``AWS_INSTANCE_TYPE`` - The aws instance type.  Defaults to ``t1.micro``.

   You can check if they are set correctly by doing something like

   ::

      echo $AWS_ACCESS_KEY

6. Do the magic!

   ::

      vagrant up --provider=aws


   If it stalls indefinitely on ``[default] Waiting for SSH to become
   available...``, Double check your default security zone on AWS
   includes rights to SSH (port 22) to your container.

   If you have an advanced AWS setup, you might want to have a look at
   https://github.com/mitchellh/vagrant-aws

7. Connect to your machine

   .. code-block:: bash

      vagrant ssh

8. Your first command

   Now you are in the VM, run docker

   .. code-block:: bash

      sudo docker


Continue with the :ref:`hello_world` example.
