# Changelog

## 0.2.0 (dev)
 - Fix Vagrant in windows and OSX
 - Fix TTY behavior
 - Fix attach/detach/run behavior
 - Fix memory/fds leaks
 - Fix various race conditions
 - Fix `docker diff` for removed files
 - Fix `docker stop` for ghost containers
 - Fix lxc 0.9 compatibility
 - Implement an escape sequence `C-p C-q` in order to detach containers in tty mode
 - Implement `-a stdin` in order to write on container's stdin while retrieving its ID
 - Implement the possiblity to choose the publicly exposed port
 - Implement progress bar for registry push/pull
 - Improve documentation
 - Improve `docker rmi` in order to remove images by name
 - Shortened containers and images IDs
 - Add cgroup capabilities detection
 - Automatically try to load AUFS module
 - Automatically create and configure a bridge `dockbr0`
 - Remove the standalone mode

## 0.1.0 (03/23/2013)
 - Open-source the project
 - Implement registry in order to push/pull images
 - Fix termcaps on Linux
 - Add the documentation
 - Add Vagrant support with Vagrantfile
 - Add unit tests
 - Add repository/tags to ease the image management
 - Improve the layer implementation
