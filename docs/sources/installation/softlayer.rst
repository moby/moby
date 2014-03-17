:title: Installation on IBM SoftLayer 
:description: Please note this project is currently under heavy development. It should not be used in production. 
:keywords: IBM SoftLayer, virtualization, cloud, docker, documentation, installation

IBM SoftLayer
=============

.. include:: install_header.inc

There are several ways to install Docker on IBM SoftLayer, but probably the simplest way is the following:

IBM SoftLayer QuickStart
-------------------------

1. Create an `IBM SoftLayer account <https://www.softlayer.com/cloudlayer/>`_.
2. Log in to the `SoftLayer Console <https://control.softlayer.com/devices/>`_.
3. Go to `Order Hourly Computing Instance <https://manage.softlayer.com/Sales/orderHourlyComputingInstance>`_ on your SoftLayer Console.
4. Create a new CCI using the default values for all the fields and choose:

- *First Available* as ``Datacenter`` and 
- *Ubuntu Linux 12.04 LTS Precise Pangolin - Minimal Install (64 bit)* as ``Operating System``.

5. Click the *Continue Your Order* button at the bottom right and select *Go to checkout*.
6. Insert the required *User Metadata* and place the order.
7. Then continue with the :ref:`ubuntu_linux` instructions.

Continue with the :ref:`hello_world` example.