page_title: Kitematic User Guide: Intro & Overview
page_description: Documentation that provides an overview of Kitematic and installation instructions
page_keywords: docker, documentation, about, technology, kitematic, gui

# Installing Kitematic

You install Kitematic much the same way you install any application on a Mac or
Windows PC: download an image and run an installer.

## Download Kitematic

[Download the Kitematic zip file](https://kitematic.com/download/), unzip the
file by double-clicking it, and then double-click the application to run it.
You'll probably also want to put the application in your Applications folder.

## Initial Setup

Opening Kitematic for the first time sets up everything you need to run Docker
containers. If you don't already have VirtualBox installed, Kitematic will
download and install the latest version.

![Installing](./assets/installing.png)

All Done! Within a minute you should be ready to start running your first
container!

![containers](./assets/containers.png)

## Technical Details

Kitematic is a self-contained .app, with a two exceptions:

- It will install VirtualBox if it's not already installed.
- It copies the `docker` and `docker-machine` binaries to `/usr/local/bin` for
  convenience.

### Why does Kitematic need my root password?

Kitematic needs your root password for two reasons:

- Installing VirtualBox requires root as it includes Mac OS X kernel extensions.
- Copying `docker` and `docker-machine` to `/usr/local/bin` may require root
  permission if the default permissions for this directory have been changed
  prior to installing Kitematic.

## Next Steps

For information about using Kitematic, take a look at the [User Guide](./userguide.md).
