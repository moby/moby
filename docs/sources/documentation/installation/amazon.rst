Amazon EC2
==========



Installation
------------

Install vagrant from http://www.vagrantup.com/ (or use your package manager)

clone the repo


Docker can be installed with Vagrant on Amazon EC2, using Vagrant 1.1 is required for EC2, but deploying is as simple as:

::

    $ export AWS_ACCESS_KEY_ID=xxx \
        AWS_SECRET_ACCESS_KEY=xxx \
        AWS_KEYPAIR_NAME=xxx \
        AWS_SSH_PRIVKEY=xxx

::

    $ vagrant plugin install vagrant-aws

::

    $ vagrant up --provider=aws

The environment variables are:

* ``AWS_ACCESS_KEY_ID`` - The API key used to make requests to AWS
* ``AWS_SECRET_ACCESS_KEY`` - The secret key to make AWS API requests
* ``AWS_KEYPAIR_NAME`` - The ID of the keypair used for this EC2 instance
* ``AWS_SSH_PRIVKEY`` - The path to the private key for the named keypair


Make sure your default security zone on AWS includes rights to SSH to your container. Otherwise access will
fail silently.


.. code-block:: bash

    vagrant ssh

Now you are in the VM, run docker

.. code-block:: bash

    docker


Continue with the :ref:`hello_world` example.