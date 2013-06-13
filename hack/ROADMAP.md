# Docker: what's next?

This document is a high-level overview of where we want to take Docker next.
It is a curated selection of planned improvements which are either important, difficult, or both.

For a more complete view of planned and requested improvements, see [the Github issues](https://github.com/dotcloud/docker/issues).

Tu suggest changes to the roadmap, including additions, please write the change as if it were already in effect, and make a pull request.

Broader kernel support
----------------------

Our goal is to make Docker run everywhere, but currently Docker requires [Linux version 3.8 or higher with lxc and aufs support](http://docs.docker.io/en/latest/installation/kernel.html). If you're deploying new machines for the purpose of running Docker, this is a fairly easy requirement to meet.
However, if you're adding Docker to an existing deployment, you may not have the flexibility to update and patch the kernel.

Expanding Docker's kernel support is a priority. This includes running on older kernel versions,
but also on kernels with no AUFS support, or with incomplete lxc capabilities.


Cross-architecture support
--------------------------

Our goal is to make Docker run everywhere. However currently Docker only runs on x86_64 systems.
We plan on expanding architecture support, so that Docker containers can be created and used on more architectures.


Even more integrations
----------------------

We want Docker to be the secret ingredient that makes your existing tools more awesome.
Thanks to this philosophy, Docker has already been integrated with
[Puppet](http://forge.puppetlabs.com/garethr/docker),  [Chef](http://www.opscode.com/chef),
[Openstack Nova](https://github.com/dotcloud/openstack-docker), [Jenkins](https://github.com/georgebashi/jenkins-docker-plugin),
[DotCloud sandbox](http://github.com/dotcloud/sandbox), [Pallet](https://github.com/pallet/pallet-docker),
[Strider CI](http://blog.frozenridge.co/next-generation-continuous-integration-deployment-with-dotclouds-docker-and-strider/)
and even [Heroku buildpacks](https://github.com/progrium/buildstep).

Expect Docker to integrate with even more of your favorite tools going forward, including:

* Alternative storage backends such as ZFS, LVM or [BTRFS](github.com/dotcloud/docker/issues/443)
* Alternative containerization backends such as [OpenVZ](http://openvz.org), Solaris Zones, BSD Jails and even plain Chroot.
* Process managers like [Supervisord](http://supervisord.org/), [Runit](http://smarden.org/runit/), [Gaffer](https://gaffer.readthedocs.org/en/latest/#gaffer) and [Systemd](http://www.freedesktop.org/wiki/Software/systemd/)
* Build and integration tools like Make, Maven, Scons, Jenkins, Buildbot and Cruise Control.
* Configuration management tools like [Puppet](http://puppetlabs.com), [Chef](http://www.opscode.com/chef/) and [Salt](http://saltstack.org)
* Personal development environments like [Vagrant](http://vagrantup.com), [Boxen](http://boxen.github.com/), [Koding](http://koding.com) and [Cloud9](http://c9.io).
* Orchestration tools like [Zookeeper](http://zookeeper.apache.org/), [Mesos](http://incubator.apache.org/mesos/) and [Galaxy](https://github.com/ning/galaxy)
* Infrastructure deployment tools like [Openstack](http://openstack.org), [Apache Cloudstack](http://apache.cloudstack.org), [Ganeti](https://code.google.com/p/ganeti/)


Plugin API
----------

We want Docker to run everywhere, and to integrate with every devops tool.
Those are ambitious goals, and the only way to reach them is with the Docker community.
For the community to participate fully, we need an API which allows Docker to be deeply and easily customized.

We are working on a plugin API which will make Docker very, very customization-friendly.
We believe it will facilitate the integrations listed above - and many more we didn't even think about.

Let us know if you want to start playing with the API before it's generally available.


Externally mounted volumes
--------------------------

In 0.3 we [introduced data volumes](https://github.com/dotcloud/docker/wiki/Docker-0.3.0-release-note%2C-May-6-2013#data-volumes),
a great mechanism for manipulating persistent data such as database files, log files, etc.

Data volumes can be shared between containers, a powerful capability [which allows many advanced use cases](http://docs.docker.io/en/latest/examples/couchdb_data_volumes.html). In the future it will also be possible to share volumes between a container and the underlying host. This will make certain scenarios much easier, such as using a high-performance storage backend for your production database,
making live development changes available to a container, etc.


Better documentation
--------------------

We believe that great documentation is worth 10 features. We are often told that "Docker's documentation is great for a 2-month old project".
Our goal is to make it great, period.

If you have feedback on how to improve our documentation, please get in touch by replying to this email,
or by [filing an issue](https://github.com/dotcloud/docker/issues). We always appreciate it!


Production-ready
----------------

Docker is still alpha software, and not suited for production.
We are working hard to get there, and we are confident that it will be possible within a few months.


Advanced port redirections
--------------------------

Docker currently supports 2 flavors of port redirection: STATIC->STATIC (eg. "redirect public port 80 to private port 80")
and RANDOM->STATIC (eg. "redirect any public port to private port 80").

With these 2 flavors, docker can support the majority of backend programs out there. But some applications have more exotic
requirements, generally to implement custom clustering techniques. These applications include Hadoop, MongoDB, Riak, RabbitMQ,
Disco, and all programs relying on Erlang's OTP.

To support these applications, Docker needs to support more advanced redirection flavors, including:

* RANDOM->RANDOM
* STATIC1->STATIC2

These flavors should be implemented without breaking existing semantics, if at all possible.
