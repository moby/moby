:title: FAQ
:description: Most frequently asked questions.
:keywords: faq, questions, documentation, docker

FAQ
===


Most frequently asked questions.
--------------------------------

How much does Docker cost?
..........................

   Docker is 100% free, it is open source, so you can use it without paying.

What open source license are you using?
.......................................

   We are using the Apache License Version 2.0, see it here:
   https://github.com/dotcloud/docker/blob/master/LICENSE

Does Docker run on Mac OS X or Windows?
.......................................

   Not at this time, Docker currently only runs on Linux, but you can
   use VirtualBox to run Docker in a virtual machine on your box, and
   get the best of both worlds. Check out the
   :ref:`install_using_vagrant` and :ref:`windows` installation
   guides.

How do containers compare to virtual machines?
..............................................

   They are complementary. VMs are best used to allocate chunks of
   hardware resources. Containers operate at the process level, which
   makes them very lightweight and perfect as a unit of software
   delivery.

What does Docker add to just plain LXC?
.......................................

   Docker is not a replacement for LXC. "LXC" refers to capabilities
   of the Linux kernel (specifically namespaces and control groups)
   which allow sandboxing processes from one another, and controlling
   their resource allocations.  On top of this low-level foundation of
   kernel features, Docker offers a high-level tool with several
   powerful functionalities:

   * *Portable deployment across machines.* 
      Docker defines a format for bundling an application and all its
      dependencies into a single object which can be transferred to
      any Docker-enabled machine, and executed there with the
      guarantee that the execution environment exposed to the
      application will be the same. LXC implements process sandboxing,
      which is an important pre-requisite for portable deployment, but
      that alone is not enough for portable deployment. If you sent me
      a copy of your application installed in a custom LXC
      configuration, it would almost certainly not run on my machine
      the way it does on yours, because it is tied to your machine's
      specific configuration: networking, storage, logging, distro,
      etc. Docker defines an abstraction for these machine-specific
      settings, so that the exact same Docker container can run -
      unchanged - on many different machines, with many different
      configurations.

   * *Application-centric.* 
      Docker is optimized for the deployment of applications, as
      opposed to machines. This is reflected in its API, user
      interface, design philosophy and documentation. By contrast, the
      ``lxc`` helper scripts focus on containers as lightweight
      machines - basically servers that boot faster and need less
      RAM. We think there's more to containers than just that.

   * *Automatic build.* 
      Docker includes :ref:`a tool for developers to automatically
      assemble a container from their source code <dockerbuilder>`,
      with full control over application dependencies, build tools,
      packaging etc. They are free to use ``make, maven, chef, puppet,
      salt,`` Debian packages, RPMs, source tarballs, or any
      combination of the above, regardless of the configuration of the
      machines.

   * *Versioning.* 
      Docker includes git-like capabilities for tracking successive
      versions of a container, inspecting the diff between versions,
      committing new versions, rolling back etc. The history also
      includes how a container was assembled and by whom, so you get
      full traceability from the production server all the way back to
      the upstream developer. Docker also implements incremental
      uploads and downloads, similar to ``git pull``, so new versions
      of a container can be transferred by only sending diffs.

   * *Component re-use.* 
      Any container can be used as a :ref:`"base image"
      <base_image_def>` to create more specialized components. This
      can be done manually or as part of an automated build. For
      example you can prepare the ideal Python environment, and use it
      as a base for 10 different applications. Your ideal Postgresql
      setup can be re-used for all your future projects. And so on.

   * *Sharing.*
      Docker has access to a `public registry
      <http://index.docker.io>`_ where thousands of people have
      uploaded useful containers: anything from Redis, CouchDB,
      Postgres to IRC bouncers to Rails app servers to Hadoop to base
      images for various Linux distros. The :ref:`registry
      <registryindexspec>` also includes an official "standard
      library" of useful containers maintained by the Docker team. The
      registry itself is open-source, so anyone can deploy their own
      registry to store and transfer private containers, for internal
      server deployments for example.

   * *Tool ecosystem.* 
      Docker defines an API for automating and customizing the
      creation and deployment of containers. There are a huge number
      of tools integrating with Docker to extend its
      capabilities. PaaS-like deployment (Dokku, Deis, Flynn),
      multi-node orchestration (Maestro, Salt, Mesos, Openstack Nova),
      management dashboards (docker-ui, Openstack Horizon, Shipyard),
      configuration management (Chef, Puppet), continuous integration
      (Jenkins, Strider, Travis), etc. Docker is rapidly establishing
      itself as the standard for container-based tooling.

Can I help by adding some questions and answers?
................................................

   Definitely! You can fork `the repo`_ and edit the documentation sources.


Where can I find more answers?
..............................

    You can find more answers on:

    * `Docker user mailinglist`_
    * `Docker developer mailinglist`_
    * `IRC, docker on freenode`_
    * `Github`_
    * `Ask questions on Stackoverflow`_
    * `Join the conversation on Twitter`_


    .. _Docker user mailinglist: https://groups.google.com/d/forum/docker-user
    .. _Docker developer mailinglist: https://groups.google.com/d/forum/docker-dev
    .. _the repo: http://www.github.com/dotcloud/docker
    .. _IRC, docker on freenode: irc://chat.freenode.net#docker
    .. _Github: http://www.github.com/dotcloud/docker
    .. _Ask questions on Stackoverflow: http://stackoverflow.com/search?q=docker
    .. _Join the conversation on Twitter: http://twitter.com/docker


Looking for something else to read? Checkout the :ref:`hello_world` example.
