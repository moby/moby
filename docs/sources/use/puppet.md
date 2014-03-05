Using Puppet[¶](#using-puppet "Permalink to this headline")
===========================================================

Note

Please note this is a community contributed installation path. The only
‘official’ installation is using the
[*Ubuntu*](../../installation/ubuntulinux/#ubuntu-linux) installation
path. This version may sometimes be out of date.

Requirements[¶](#requirements "Permalink to this headline")
-----------------------------------------------------------

To use this guide you’ll need a working installation of Puppet from
[Puppetlabs](https://www.puppetlabs.com) .

The module also currently uses the official PPA so only works with
Ubuntu.

Installation[¶](#installation "Permalink to this headline")
-----------------------------------------------------------

The module is available on the [Puppet
Forge](https://forge.puppetlabs.com/garethr/docker/) and can be
installed using the built-in module tool.

    puppet module install garethr/docker

It can also be found on
[GitHub](https://www.github.com/garethr/garethr-docker) if you would
rather download the source.

Usage[¶](#usage "Permalink to this headline")
---------------------------------------------

The module provides a puppet class for installing Docker and two defined
types for managing images and containers.

### Installation[¶](#id1 "Permalink to this headline")

    include 'docker'

### Images[¶](#images "Permalink to this headline")

The next step is probably to install a Docker image. For this, we have a
defined type which can be used like so:

    docker::image { 'ubuntu': }

This is equivalent to running:

    docker pull ubuntu

Note that it will only be downloaded if an image of that name does not
already exist. This is downloading a large binary so on first run can
take a while. For that reason this define turns off the default 5 minute
timeout for the exec type. Note that you can also remove images you no
longer need with:

    docker::image { 'ubuntu':
      ensure => 'absent',
    }

### Containers[¶](#containers "Permalink to this headline")

Now you have an image where you can run commands within a container
managed by Docker.

    docker::run { 'helloworld':
      image   => 'ubuntu',
      command => '/bin/sh -c "while true; do echo hello world; sleep 1; done"',
    }

This is equivalent to running the following command, but under upstart:

    docker run -d ubuntu /bin/sh -c "while true; do echo hello world; sleep 1; done"

Run also contains a number of optional parameters:

    docker::run { 'helloworld':
      image        => 'ubuntu',
      command      => '/bin/sh -c "while true; do echo hello world; sleep 1; done"',
      ports        => ['4444', '4555'],
      volumes      => ['/var/lib/couchdb', '/var/log'],
      volumes_from => '6446ea52fbc9',
      memory_limit => 10485760, # bytes
      username     => 'example',
      hostname     => 'example.com',
      env          => ['FOO=BAR', 'FOO2=BAR2'],
      dns          => ['8.8.8.8', '8.8.4.4'],
    }

Note that ports, env, dns and volumes can be set with either a single
string or as above with an array of values.
