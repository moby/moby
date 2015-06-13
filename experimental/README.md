# Docker Experimental Features 

This page contains a list of features in the Docker engine which are
experimental. Experimental features are **not** ready for production. They are
provided for test and evaluation in your sandbox environments.  

The information below describes each feature and the GitHub pull requests and
issues associated with it. If necessary, links are provided to additional
documentation on an issue.  As an active Docker user and community member,
please feel free to provide any feedback on these features you wish.

## Install Docker experimental 

Unlike the regular Docker binary, the experimental channels is built and updated nightly on TO.BE.ANNOUNCED. From one day to the next, new features may appear, while existing experimental features may be refined or entirely removed.

1. Verify that you have `wget` installed.

        $ which wget

    If `wget` isn't installed, install it after updating your manager:

        $ sudo apt-get update
        $ sudo apt-get install wget

2. Get the latest Docker package.

        $ wget -qO- https://experimental.docker.com/ | sh

    The system prompts you for your `sudo` password. Then, it downloads and
    installs Docker and its dependencies.

	>**Note**: If your company is behind a filtering proxy, you may find that the
	>`apt-key`
	>command fails for the Docker repo during installation. To work around this,
	>add the key directly using the following:
	>
	>       $ wget -qO- https://experimental.docker.com/gpg | sudo apt-key add -

3. Verify `docker` is installed correctly.

        $ sudo docker run hello-world

    This command downloads a test image and runs it in a container.

## Current experimental features

* [Support for Docker plugins](plugins.md)
* [Volume plugins](plugins_volume.md)

## How to comment on an experimental feature

Each feature's documentation includes a list of proposal pull requests or PRs associated with the feature. If you want to comment on or suggest a change to a feature, please add it to the existing feature PR.  

Issues or problems with a feature? Inquire for help on the `#docker` IRC channel or in on the [Docker Google group](https://groups.google.com/forum/#!forum/docker-user).  
