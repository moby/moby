# Changelog

## 0.4.8 (2013-07-01)
 + Builder: New build operation ENTRYPOINT adds an executable entry point to the container.
 - Runtime: Fix a bug which caused 'docker run -d' to no longer print the container ID.
 - Tests: Fix issues in the test suite

## 0.4.7 (2013-06-28)
 * Registry: easier push/pull to a custom registry
 * Remote API: the progress bar updates faster when downloading and uploading large files
 - Remote API: fix a bug in the optional unix socket transport
 * Runtime: improve detection of kernel version
 + Runtime: host directories can be mounted as volumes with 'docker run -b'
 - Runtime: fix an issue when only attaching to stdin
 * Runtime: use 'tar --numeric-owner' to avoid uid mismatch across multiple hosts
 * Hack: improve test suite and dev environment
 * Hack: remove dependency on unit tests on 'os/user'
 + Documentation: add terminology section

## 0.4.6 (2013-06-22)
 - Runtime: fix a bug which caused creation of empty images (and volumes) to crash.

## 0.4.5 (2013-06-21)
 + Builder: 'docker build git://URL' fetches and builds a remote git repository
 * Runtime: 'docker ps -s' optionally prints container size
 * Tests: Improved and simplified
 - Runtime: fix a regression introduced in 0.4.3 which caused the logs command to fail.
 - Builder: fix a regression when using ADD with single regular file.

## 0.4.4 (2013-06-19)
 - Builder: fix a regression introduced in 0.4.3 which caused builds to fail on new clients.

## 0.4.3 (2013-06-19)
 + Builder: ADD of a local file will detect tar archives and unpack them
 * Runtime: Remove bsdtar dependency
 * Runtime: Add unix socket and multiple -H support
 * Runtime: Prevent rm of running containers
 * Runtime: Use go1.1 cookiejar
 * Builder: ADD improvements: use tar for copy + automatically unpack local archives
 * Builder: ADD uses tar/untar for copies instead of calling 'cp -ar'
 * Builder: nicer output for 'docker build'
 * Builder: fixed the behavior of ADD to be (mostly) reverse-compatible, predictable and well-documented.
 * Client: HumanReadable ProgressBar sizes in pull
 * Client: Fix docker version's git commit output
 * API: Send all tags on History API call
 * API: Add tag lookup to history command. Fixes #882
 - Runtime: Fix issue detaching from running TTY container
 - Runtime: Forbid parralel push/pull for a single image/repo. Fixes #311
 - Runtime: Fix race condition within Run command when attaching.
 - Builder: fix a bug which caused builds to fail if ADD was the first command
 - Documentation: fix missing command in irc bouncer example

## 0.4.2 (2013-06-17)
 - Packaging: Bumped version to work around an Ubuntu bug

## 0.4.1 (2013-06-17)
 + Remote Api: Add flag to enable cross domain requests
 + Remote Api/Client: Add images and containers sizes in docker ps and docker images
 + Runtime: Configure dns configuration host-wide with 'docker -d -dns'
 + Runtime: Detect faulty DNS configuration and replace it with a public default
 + Runtime: allow docker run <name>:<id>
 + Runtime: you can now specify public port (ex: -p 80:4500)
 * Client: allow multiple params in inspect
 * Client: Print the container id before the hijack in `docker run`
 * Registry: add regexp check on repo's name
 * Registry: Move auth to the client
 * Runtime: improved image removal to garbage-collect unreferenced parents
 * Vagrantfile: Add the rest api port to vagrantfile's port_forward
 * Upgrade to Go 1.1
 - Builder: don't ignore last line in Dockerfile when it doesn't end with \n
 - Registry: Remove login check on pull

## 0.4.0 (2013-06-03)
 + Introducing Builder: 'docker build' builds a container, layer by layer, from a source repository containing a Dockerfile
 + Introducing Remote API: control Docker programmatically using a simple HTTP/json API
 * Runtime: various reliability and usability improvements

## 0.3.4 (2013-05-30)
 + Builder: 'docker build' builds a container, layer by layer, from a source repository containing a Dockerfile
 + Builder: 'docker build -t FOO' applies the tag FOO to the newly built container.
 + Runtime: interactive TTYs correctly handle window resize
 * Runtime: fix how configuration is merged between layers
 + Remote API: split stdout and stderr on 'docker run'
 + Remote API: optionally listen on a different IP and port (use at your own risk)
 * Documentation: improved install instructions.

## 0.3.3 (2013-05-23)
 - Registry: Fix push regression
 - Various bugfixes

## 0.3.2 (2013-05-09)
 * Runtime: Store the actual archive on commit
 * Registry: Improve the checksum process
 * Registry: Use the size to have a good progress bar while pushing
 * Registry: Use the actual archive if it exists in order to speed up the push
 - Registry: Fix error 400 on push

## 0.3.1 (2013-05-08)
 + Builder: Implement the autorun capability within docker builder
 + Builder: Add caching to docker builder
 + Builder: Add support for docker builder with native API as top level command
 + Runtime: Add go version to debug infos
 + Builder: Implement ENV within docker builder
 + Registry: Add docker search top level command in order to search a repository
 + Images: output graph of images to dot (graphviz)
 + Documentation: new introduction and high-level overview
 + Documentation: Add the documentation for docker builder
 + Website: new high-level overview
 - Makefile: Swap "go get" for "go get -d", especially to compile on go1.1rc
 - Images: fix ByParent function
 - Builder: Check the command existance prior create and add Unit tests for the case
 - Registry: Fix pull for official images with specific tag
 - Registry: Fix issue when login in with a different user and trying to push
 - Documentation: CSS fix for docker documentation to make REST API docs look better.
 - Documentation: Fixed CouchDB example page header mistake
 - Documentation: fixed README formatting
 * Registry: Improve checksum - async calculation
 * Runtime: kernel version - don't show the dash if flavor is empty
 * Documentation: updated www.docker.io website.
 * Builder: use any whitespaces instead of tabs
 * Packaging: packaging ubuntu; issue #510: Use goland-stable PPA package to build docker

## 0.3.0 (2013-05-06)
 + Registry: Implement the new registry
 + Documentation: new example: sharing data between 2 couchdb databases
 - Runtime: Fix the command existance check
 - Runtime: strings.Split may return an empty string on no match
 - Runtime: Fix an index out of range crash if cgroup memory is not
 * Documentation: Various improvments
 * Vagrant: Use only one deb line in /etc/apt

## 0.2.2 (2013-05-03)
 + Support for data volumes ('docker run -v=PATH')
 + Share data volumes between containers ('docker run -volumes-from')
 + Improved documentation
 * Upgrade to Go 1.0.3
 * Various upgrades to the dev environment for contributors

## 0.2.1 (2013-05-01)
 + 'docker commit -run' bundles a layer with default runtime options: command, ports etc.
 * Improve install process on Vagrant
 + New Dockerfile operation: "maintainer"
 + New Dockerfile operation: "expose"
 + New Dockerfile operation: "cmd"
 + Contrib script to build a Debian base layer
 + 'docker -d -r': restart crashed containers at daemon startup
 * Runtime: improve test coverage

## 0.2.0 (2013-04-23)
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
