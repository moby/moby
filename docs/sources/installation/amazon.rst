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

   * Open http://cloud-images.ubuntu.com/locator/ec2/
   * Enter ``amd64 precise`` in the search field (it will search as you
     type)
   * Pick an image by clicking on the image name. *An EBS-enabled
     image will let you t1.micro instance.* Clicking on the image name
     will take you to your AWS Console.

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

Installing with ``get.docker.io`` (as above) will create a service
named ``dockerd``. You may want to set up a :ref:`docker group
<dockergroup>` and add the *ubuntu* user to it so that you don't have
to use ``sudo`` for every Docker command.

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

   ::

       export AWS_ACCESS_KEY_ID=xxx
       export AWS_SECRET_ACCESS_KEY=xxx
       export AWS_KEYPAIR_NAME=xxx
       export AWS_SSH_PRIVKEY=xxx

   The environment variables are:

   * ``AWS_ACCESS_KEY_ID`` - The API key used to make requests to AWS
   * ``AWS_SECRET_ACCESS_KEY`` - The secret key to make AWS API requests
   * ``AWS_KEYPAIR_NAME`` - The name of the keypair used for this EC2 instance
   * ``AWS_SSH_PRIVKEY`` - The path to the private key for the named
     keypair, for example ``~/.ssh/docker.pem``

   You can check if they are set correctly by doing something like

   ::

      echo $AWS_ACCESS_KEY_ID

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
