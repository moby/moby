:title: Installation on Google Cloud Platform
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, installation, google, Google Compute Engine, Google Cloud Platform

`Google Cloud Platform <https://cloud.google.com/>`_
====================================================

.. include:: install_header.inc

.. _googlequickstart:

`Compute Engine <https://developers.google.com/compute>`_ QuickStart for `Debian <https://www.debian.org>`_
-----------------------------------------------------------------------------------------------------------

1. Go to `Google Cloud Console <https://cloud.google.com/console>`_ and create a new Cloud Project with billing enabled.

2. Download and configure the `Google Cloud SDK <https://developers.google.com/cloud/sdk/>`_ to use your project with the following commands:

.. code-block:: bash

    $ curl https://dl.google.com/dl/cloudsdk/release/install_google_cloud_sdk.bash | bash
    $ gcloud auth login
    Enter a cloud project id (or leave blank to not set): <google-cloud-project-id>

3. Start a new instance, select a zone close to you and the desired instance size:

.. code-block:: bash

    $ gcutil addinstance docker-playground --image=backports-debian-7
    1: europe-west1-a
    ...
    4: us-central1-b
    >>> <zone-index>
    1: machineTypes/n1-standard-1
    ...
    12: machineTypes/g1-small
    >>> <machine-type-index>

4. Connect to the instance using SSH:

.. code-block:: bash

    $ gcutil ssh docker-playground
    docker-playground:~$ 

5. Enable IP forwarding:

.. code-block:: bash

    docker-playground:~$ echo net.ipv4.ip_forward=1 | sudo tee /etc/sysctl.d/99-docker.conf 
    docker-playground:~$ sudo sysctl --system

6. Install the latest Docker release and configure it to start when the instance boots:

.. code-block:: bash

    docker-playground:~$ curl get.docker.io | bash
    docker-playground:~$ sudo update-rc.d docker defaults

7. Start a new container:

.. code-block:: bash

    docker-playground:~$ sudo docker run busybox echo 'docker on GCE \o/'
    docker on GCE \o/
