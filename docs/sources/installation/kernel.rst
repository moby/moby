.. _kernel:

Kernel Requirements
===================

  The officially supported kernel is the one recommended by the
  :ref:`ubuntu_linux` installation path. It is the one that most developers
  will use, and the one that receives the most attention from the core
  contributors. If you decide to go with a different kernel and hit a bug,
  please try to reproduce it with the official kernels first.

If for some reason you cannot or do not want to use the "official" kernels,
here is some technical background about the features (both optional and
mandatory) that docker needs to run successfully.

In short, you need kernel version 3.8 (or above), compiled to include
`AUFS support <http://aufs.sourceforge.net/>`_. Of course, you need to
enable cgroups and namespaces.


Namespaces and Cgroups
----------------------

You need to enable namespaces and cgroups, to the extend of what is needed
to run LXC containers. Technically, while namespaces have been introduced
in the early 2.6 kernels, we do not advise to try any kernel before 2.6.32
to run LXC containers. Note that 2.6.32 has some documented issues regarding
network namespace setup and teardown; those issues are not a risk if you
run containers in a private environment, but can lead to denial-of-service
attacks if you want to run untrusted code in your containers. For more details,
see `[LP#720095 <https://bugs.launchpad.net/ubuntu/+source/linux/+bug/720095>`_.

Kernels 2.6.38, and every version since 3.2, have been deployed successfully
to run containerized production workloads. Feature-wise, there is no huge
improvement between 2.6.38 and up to 3.6 (as far as docker is concerned!).

Starting with version 3.7, the kernel has basic support for
`Checkpoint/Restore In Userspace <http://criu.org/>`_, which is not used by
docker at this point, but allows to suspend the state of a container to
disk and resume it later.

Version 3.8 provides improvements in stability, which are deemed necessary
for the operation of docker. Versions 3.2 to 3.5 have been shown to
exhibit a reproducible bug (for more details, see issue
`#407 <https://github.com/dotcloud/docker/issues/407>`_).

Version 3.8 also brings better support for the
`setns() syscall <http://lwn.net/Articles/531381/>`_ -- but this should not
be a concern since docker does not leverage on this feature for now.

If you want a technical overview about those concepts, you might
want to check those articles on dotCloud's blog:
`about namespaces <http://blog.dotcloud.com/under-the-hood-linux-kernels-on-dotcloud-part>`_
and `about cgroups <http://blog.dotcloud.com/kernel-secrets-from-the-paas-garage-part-24-c>`_.


Important Note About Pre-3.8 Kernels
------------------------------------

As mentioned above, kernels before 3.8 are not stable when used with docker.
In some circumstances, you will experience kernel "oopses", or even crashes.
The symptoms include:

- a container being killed in the middle of an operation (e.g. an ``apt-get``
  command doesn't complete);
- kernel messages including mentioning calls to ``mntput`` or
  ``d_hash_and_lookup``;
- kernel crash causing the machine to freeze for a few minutes, or even
  completely.

While it is still possible to use older kernels for development, it is
really not advised to do so.

Docker checks the kernel version when it starts, and emits a warning if it
detects something older than 3.8.

See issue `#407 <https://github.com/dotcloud/docker/issues/407>`_ for details.


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

    cgroup_enable=memory swapaccount

On Debian or Ubuntu systems, if you use the default GRUB bootloader, you can
add those parameters by editing ``/etc/default/grub`` and extending
``GRUB_CMDLINE_LINUX``. Look for the following line::

    GRUB_CMDLINE_LINUX=""

And replace it by the following one::

    GRUB_CMDLINE_LINUX="cgroup_enable=memory swapaccount"

Then run ``update-grub``, and reboot.


AUFS
----

Docker currently relies on AUFS, an unioning filesystem.
While AUFS is included in the kernels built by the Debian and Ubuntu
distributions, is not part of the standard kernel. This means that if
you decide to roll your own kernel, you will have to patch your
kernel tree to add AUFS. The process is documented on
`AUFS webpage <http://aufs.sourceforge.net/>`_.

Note: the AUFS patch is fairly intrusive, but for the record, people have
successfully applied GRSEC and AUFS together, to obtain hardened production
kernels.

If you want more information about that topic, there is an
`article about AUFS on dotCloud's blog 
<http://blog.dotcloud.com/kernel-secrets-from-the-paas-garage-part-34-a>`_.


BTRFS, ZFS, OverlayFS...
------------------------

There is ongoing development on docker, to implement support for
`BTRFS <http://en.wikipedia.org/wiki/Btrfs>`_
(see github issue `#443 <https://github.com/dotcloud/docker/issues/443>`_).

People have also showed interest for `ZFS <http://en.wikipedia.org/wiki/ZFS>`_
(using e.g. `ZFS-on-Linux <http://zfsonlinux.org/>`_) and OverlayFS.
The latter is functionally close to AUFS, and it might end up being included
in the stock kernel; so it's a strong candidate!

Would you like to `contribute
<https://github.com/dotcloud/docker/blob/master/CONTRIBUTING.md>`_
support for your favorite filesystem?
