:title: Installation on Amazon EC2
:description: Docker installation on Amazon EC2 with a single vagrant command. Vagrant 1.1 or higher is required.
:keywords: amazon ec2, virtualization, cloud, docker, documentation, installation

Using Vagrant (Amazon EC2)
==========================

  Please note this is a community contributed installation path. The only 'official' installation is using the
  :ref:`ubuntu_linux` installation path. This version may sometimes be out of date.


Installation
------------

Docker can now be installed on Amazon EC2 with a single vagrant command. Vagrant 1.1 or higher is required.

1. Install vagrant from http://www.vagrantup.com/ (or use your package manager)
2. Install the vagrant aws plugin

   ::

       vagrant plugin install vagrant-aws


3. Get the docker sources, this will give you the latest Vagrantfile.

   ::

      git clone https://github.com/dotcloud/docker.git


4. Check your AWS environment.

   Create a keypair specifically for EC2, give it a name and save it to your disk. *I usually store these in my ~/.ssh/ folder*.

   Check that your default security group has an inbound rule to accept SSH (port 22) connections.



5. Inform Vagrant of your settings

   Vagrant will read your access credentials from your environment, so we need to set them there first. Make sure
   you have everything on amazon aws setup so you can (manually) deploy a new image to EC2.

   ::

       export AWS_ACCESS_KEY_ID=xxx
       export AWS_SECRET_ACCESS_KEY=xxx
       export AWS_KEYPAIR_NAME=xxx
       export AWS_SSH_PRIVKEY=xxx

   The environment variables are:

   * ``AWS_ACCESS_KEY_ID`` - The API key used to make requests to AWS
   * ``AWS_SECRET_ACCESS_KEY`` - The secret key to make AWS API requests
   * ``AWS_KEYPAIR_NAME`` - The name of the keypair used for this EC2 instance
   * ``AWS_SSH_PRIVKEY`` - The path to the private key for the named keypair, for example ``~/.ssh/docker.pem``

   You can check if they are set correctly by doing something like

   ::

      echo $AWS_ACCESS_KEY_ID

6. Do the magic!

   ::

      vagrant up --provider=aws


   If it stalls indefinitely on ``[default] Waiting for SSH to become available...``, Double check your default security
   zone on AWS includes rights to SSH (port 22) to your container.

   If you have an advanced AWS setup, you might want to have a look at https://github.com/mitchellh/vagrant-aws

7. Connect to your machine

   .. code-block:: bash

      vagrant ssh

8. Your first command

   Now you are in the VM, run docker

   .. code-block:: bash

      docker


Continue with the :ref:`hello_world` example.