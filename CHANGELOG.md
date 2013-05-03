# Changelog

## 0.2.2 (2012-05-03)
 + Support for data volumes ('docker run -v=PATH')
 + Share data volumes between containers ('docker run -volumes-from')
 + Improved documentation
 * Upgrade to Go 1.0.3
 * Various upgrades to the dev environment for contributors

## 0.2.1 (2012-05-01)
 + 'docker commit -run' bundles a layer with default runtime options: command, ports etc. 
 * Improve install process on Vagrant
 + New Dockerfile operation: "maintainer"
 + New Dockerfile operation: "expose"
 + New Dockerfile operation: "cmd"
 + Contrib script to build a Debian base layer
 + 'docker -d -r': restart crashed containers at daemon startup
 * Runtime: improve test coverage

## 0.2.0 (2012-04-23)
 - Runtime: ghost containers can be killed and waited for
 * Documentation: update install intructions
 - Packaging: fix Vagrantfile
 - Development: automate releasing binaries and ubuntu packages
 + Add a changelog
 - Various bugfixes


## 0.1.8 (2013-04-22)
 - Dynamically detect cgroup capabilities
 - Issue stability warning on kernels <3.8
 - 'docker push' buffers on disk instead of memory
 - Fix 'docker diff' for removed files
 - Fix 'docker stop' for ghost containers 
 - Fix handling of pidfile
 - Various bugfixes and stability improvements

## 0.1.7 (2013-04-18)
 - Container ports are available on localhost
 - 'docker ps' shows allocated TCP ports
 - Contributors can run 'make hack' to start a continuous integration VM
 - Streamline ubuntu packaging & uploading
 - Various bugfixes and stability improvements

## 0.1.6 (2013-04-17)
 - Record the author an image with 'docker commit -author'

## 0.1.5 (2013-04-17)
 - Disable standalone mode
 - Use a custom DNS resolver with 'docker -d -dns'
 - Detect ghost containers
 - Improve diagnosis of missing system capabilities
 - Allow disabling memory limits at compile time
 - Add debian packaging
 - Documentation: installing on Arch Linux 
 - Documentation: running Redis on docker
 - Fixed lxc 0.9 compatibility
 - Automatically load aufs module
 - Various bugfixes and stability improvements

## 0.1.4 (2013-04-09)
 - Full support for TTY emulation
 - Detach from a TTY session with the escape sequence `C-p C-q`
 - Various bugfixes and stability improvements
 - Minor UI improvements
 - Automatically create our own bridge interface 'docker0'

## 0.1.3 (2013-04-04)
 - Choose TCP frontend port with '-p :PORT'
 - Layer format is versioned
 - Major reliability improvements to the process manager
 - Various bugfixes and stability improvements

## 0.1.2 (2013-04-03)
 - Set container hostname with 'docker run -h'
 - Selective attach at run with 'docker run -a [stdin[,stdout[,stderr]]]'
 - Various bugfixes and stability improvements
 - UI polish
 - Progress bar on push/pull
 - Use XZ compression by default
 - Make IP allocator lazy

## 0.1.1 (2013-03-31)
 - Display shorthand IDs for convenience
 - Stabilize process management
 - Layers can include a commit message
 - Simplified 'docker attach'
 - Fixed support for re-attaching
 - Various bugfixes and stability improvements
 - Auto-download at run
 - Auto-login on push
 - Beefed up documentation

## 0.1.0 (2013-03-23)
 - First release
 - Implement registry in order to push/pull images
 - TCP port allocation
 - Fix termcaps on Linux
 - Add documentation
 - Add Vagrant support with Vagrantfile
 - Add unit tests
 - Add repository/tags to ease image management
 - Improve the layer implementation
