:title: Kernel Requirements
:description: Kernel supports
:keywords: kernel requirements, kernel support, docker, installation, cgroups, namespaces

.. _kernel:

Kernel Requirements
===================

In short, Docker has the following kernel requirements:

- Linux version 3.8 or above.

- Cgroups and namespaces must be enabled.

*Note: as of 0.7 docker no longer requires aufs. AUFS support is still available as an optional driver.*

The officially supported kernel is the one recommended by the
:ref:`ubuntu_linux` installation path. It is the one that most developers
will use, and the one that receives the most attention from the core
contributors. If you decide to go with a different kernel and hit a bug,
please try to reproduce it with the official kernels first.

If you cannot or do not want to use the "official" kernels,
here is some technical background about the features (both optional and
mandatory) that docker needs to run successfully.


Linux version 3.8 or above
--------------------------

Kernel versions 3.2 to 3.5 are not stable when used with docker.
In some circumstances, you will experience kernel "oopses", or even crashes.
The symptoms include:

- a container being killed in the middle of an operation (e.g. an ``apt-get``
  command doesn't complete);
- kernel messages including mentioning calls to ``mntput`` or
  ``d_hash_and_lookup``;
- kernel crash causing the machine to freeze for a few minutes, or even
  completely.

Additionally, kernels prior 3.4 did not implement ``reboot_pid_ns``,
which means that the ``reboot()`` syscall could reboot the host machine,
instead of terminating the container. To work around that problem,
LXC userland tools (since version 0.8) automatically drop the ``SYS_BOOT``
capability when necessary. Still, if you run a pre-3.4 kernel with pre-0.8
LXC tools, be aware that containers can reboot the whole host! This is
not something that Docker wants to address in the short term, since you
shouldn't use kernels prior 3.8 with Docker anyway.

While it is still possible to use older kernels for development, it is
really not advised to do so.

Docker checks the kernel version when it starts, and emits a warning if it
detects something older than 3.8.

See issue `#407 <https://github.com/dotcloud/docker/issues/407>`_ for details.


Cgroups and namespaces
----------------------

You need to enable namespaces and cgroups, to the extent of what is needed
to run LXC containers. Technically, while namespaces have been introduced
in the early 2.6 kernels, we do not advise to try any kernel before 2.6.32
to run LXC containers. Note that 2.6.32 has some documented issues regarding
network namespace setup and teardown; those issues are not a risk if you
run containers in a private environment, but can lead to denial-of-service
attacks if you want to run untrusted code in your containers. For more details,
see `LP#720095 <https://bugs.launchpad.net/ubuntu/+source/linux/+bug/720095>`_.

Kernels 2.6.38, and every version since 3.2, have been deployed successfully
to run containerized production workloads. Feature-wise, there is no huge
improvement between 2.6.38 and up to 3.6 (as far as docker is concerned!).




Extra Cgroup Controllers
------------------------

Most control groups can be enabled or disabled individually. For instance,
you can decide that you do not want to compile support for the CPU or memory
controller. In some cases, the feature can be enabled or disabled at boot
time. It is worth mentioning that some distributions (like Debian) disable
"expensive" features, like the memory controller, because they can have
a significant performance impact.

In the specific case of the memory cgroup, docker will detect if the cgroup
is available or not. If it's not, it will print a warning, and it won't
use the feature. If you want to enable that feature -- read on!


Memory and Swap Accounting on Debian/Ubuntu
-------------------------------------------

If you use Debian or Ubuntu kernels, and want to enable memory and swap
accounting, you must add the following command-line parameters to your kernel::

    cgroup_enable=memory swapaccount=1

On Debian or Ubuntu systems, if you use the default GRUB bootloader, you can
add those parameters by editing ``/etc/default/grub`` and extending
``GRUB_CMDLINE_LINUX``. Look for the following line::

    GRUB_CMDLINE_LINUX=""

And replace it by the following one::

    GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount=1"

Then run ``update-grub``, and reboot.
