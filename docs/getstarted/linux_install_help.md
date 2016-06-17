<!--[metadata]>
+++
aliases = ["/mac/started/"]
title = "Install Docker and run hello-world"
description = "Getting started with Docker"
keywords = ["beginner, getting started, Docker, install"]
identifier = "getstart_linux_install"
parent = "tutorial_getstart_menu"
weight="-80"
+++
<![end-metadata]-->

# Example: Install Docker on Ubuntu Linux

This installation procedure for users who are unfamiliar with package
managers, and just want to try out the Getting Started tutorial while running Docker on Linux. If you are comfortable with package managers, prefer not to use
`curl`, or have problems installing and want to troubleshoot, please use our
`apt` and `yum` <a href="https://docs.docker.com/engine/installation/"
target="_blank">repositories instead for your installation</a>.

1. Log into your Ubuntu installation as a user with `sudo` privileges.

2. Verify that you have `curl` installed.

        $ which curl

    If `curl` isn't installed, install it after updating your manager:

        $ sudo apt-get update
        $ sudo apt-get install curl

3. Get the latest Docker package.

        $ curl -fsSL https://get.docker.com/ | sh

    The system prompts you for your `sudo` password. Then, it downloads and
    installs Docker and its dependencies.

    >**Note**: If your company is behind a filtering proxy, you may find that the
    >`apt-key`
    >command fails for the Docker repo during installation. To work around this,
    >add the key directly using the following:
    >
    >       $ curl -fsSL https://get.docker.com/gpg | sudo apt-key add -
