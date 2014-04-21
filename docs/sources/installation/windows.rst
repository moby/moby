:title: Installation on Windows
:description: Please note this project is currently under heavy development. It should not be used in production.
:keywords: Docker, Docker documentation, Windows, requirements, virtualbox, boot2docker

.. _windows:

Microsoft Windows
=================

Docker can run on Windows using a virtualization platform like VirtualBox. A Linux
distribution is run inside a virtual machine and that's where Docker will run. 

Installation
------------

.. include:: install_header.inc

1. Install VirtualBox from https://www.virtualbox.org - or follow this `tutorial <http://www.slideshare.net/julienbarbier42/install-virtualbox-on-windows-7>`_.

2. Download the latest boot2docker.iso from https://github.com/boot2docker/boot2docker/releases.

3. Start VirtualBox.

4. Create a new Virtual machine with the following settings:

 - `Name: boot2docker`
 - `Type: Linux`
 - `Version: Linux 2.6 (64 bit)`
 - `Memory size: 1024 MB`
 - `Hard drive: Do not add a virtual hard drive`

5. Open the settings of the virtual machine:

   5.1. go to Storage

   5.2. click the empty slot below `Controller: IDE`

   5.3. click the disc icon on the right of `IDE Secondary Master`

   5.4. click `Choose a virtual CD/DVD disk file`

6. Browse to the path where you've saved the `boot2docker.iso`, select the `boot2docker.iso` and click open.

7. Click OK on the Settings dialog to save the changes and close the window.

8. Start the virtual machine by clicking the green start button.

9. The boot2docker virtual machine should boot now.

Running Docker
--------------

boot2docker will log you in automatically so you can start using Docker right
away.

Let's try the “hello world” example. Run

.. code-block:: bash

	docker run busybox echo hello world

This will download the small busybox image and print hello world.


Observations
------------

Persistent storage
``````````````````

The virtual machine created above lacks any persistent data storage. All images
and containers will be lost when shutting down or rebooting the VM.
