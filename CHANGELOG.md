# Changelog

## 0.7.0 (2013-11-25)

#### Notable features since 0.6.0

* Storage drivers: choose from aufs, device-mapper, or vfs.
* Standard Linux support: docker now runs on unmodified Linux kernels and all major distributions.
* Links: compose complex software stacks by connecting containers to each other.
* Container naming: organize your containers by giving them memorable names.
* Advanced port redirects: specify port redirects per interface, or keep sensitive ports private.
* Offline transfer: push and pull images to the filesystem without losing information.
* Quality: numerous bugfixes and small usability improvements. Significant increase in test coverage.

## 0.6.7 (2013-11-21)

#### Runtime

* Improved stability, fixes some race conditons
* Skip the volumes mounted when deleting the volumes of container.
* Fix layer size computation: handle hard links correctly
* Use the work Path for docker cp CONTAINER:PATH
* Fix tmp dir never cleanup
* Speedup docker ps
* More informative error message on name collisions
* Fix nameserver regex
* Always return long id's
* Fix container restart race condition
* Keep published ports on docker stop;docker start
* Fix container networking on Fedora
* Correctly express "any address" to iptables
* Fix network setup when reconnecting to ghost container
* Prevent deletion if image is used by a running container
* Lock around read operations in graph

#### RemoteAPI

* Return full ID on docker rmi

#### Client

+ Add -tree option to images
+ Offline image transfer
* Exit with status 2 on usage error and display usage on stderr
* Do not forward SIGCHLD to container
* Use string timestamp for docker events -since

#### Other

* Update to go 1.2rc5
+ Add /etc/default/docker support to upstart

## 0.6.6 (2013-11-06)

#### Runtime

* Ensure container name on register
* Fix regression in /etc/hosts
+ Add lock around write operations in graph
* Check if port is valid
* Fix restart runtime error with ghost container networking
+ Added some more colors and animals to increase the pool of generated names
* Fix issues in docker inspect
+ Escape apparmor confinement
+ Set environment variables using a file.
* Prevent docker insert to erase something
+ Prevent DNS server conflicts in CreateBridgeIface
+ Validate bind mounts on the server side
+ Use parent image config in docker build
* Fix regression in /etc/hosts

#### Client

+ Add -P flag to publish all exposed ports
+ Add -notrunc and -q flags to docker history
* Fix docker commit, tag and import usage
+ Add stars, trusted builds and library flags in docker search
* Fix docker logs with tty

#### RemoteAPI

* Make /events API send headers immediately
* Do not split last column docker top
+ Add size to history

#### Other

+ Contrib: Desktop integration. Firefox usecase.
+ Dockerfile: bump to go1.2rc3

## 0.6.5 (2013-10-29)

#### Runtime

+ Containers can now be named
+ Containers can now be linked together for service discovery
+ 'run -a', 'start -a' and 'attach' can forward signals to the container for better integration with process supervisors
+ Automatically start crashed containers after a reboot
+ Expose IP, port, and proto as separate environment vars for container links
* Allow ports to be published to specific ips
* Prohibit inter-container communication by default
- Ignore ErrClosedPipe for stdin in Container.Attach
- Remove unused field kernelVersion
* Fix issue when mounting subdirectories of /mnt in container
- Fix untag during removal of images
* Check return value of syscall.Chdir when changing working directory inside dockerinit

#### Client

- Only pass stdin to hijack when needed to avoid closed pipe errors
* Use less reflection in command-line method invocation
- Monitor the tty size after starting the container, not prior
- Remove useless os.Exit() calls after log.Fatal

#### Hack

+ Add initial init scripts library and a safer Ubuntu packaging script that works for Debian
* Add -p option to invoke debootstrap with http_proxy
- Update install.sh with $sh_c to get sudo/su for modprobe
* Update all the mkimage scripts to use --numeric-owner as a tar argument
* Update hack/release.sh process to automatically invoke hack/make.sh and bail on build and test issues

#### Other

* Documentation: Fix the flags for nc in example
* Testing: Remove warnings and prevent mount issues
- Testing: Change logic for tty resize to avoid warning in tests
- Builder: Fix race condition in docker build with verbose output
- Registry: Fix content-type for PushImageJSONIndex method
* Contrib: Improve helper tools to generate debian and Arch linux server images

## 0.6.4 (2013-10-16)

#### Runtime

- Add cleanup of container when Start() fails
* Add better comments to utils/stdcopy.go
* Add utils.Errorf for error logging
+ Add -rm to docker run for removing a container on exit
- Remove error messages which are not actually errors
- Fix `docker rm` with volumes
- Fix some error cases where a HTTP body might not be closed
- Fix panic with wrong dockercfg file
- Fix the attach behavior with -i
* Record termination time in state.
- Use empty string so TempDir uses the OS's temp dir automatically
- Make sure to close the network allocators
+ Autorestart containers by default
* Bump vendor kr/pty to commit 3b1f6487b `(syscall.O_NOCTTY)`
* lxc: Allow set_file_cap capability in container
- Move run -rm to the cli only
* Split stdout stderr
* Always create a new session for the container

#### Testing

- Add aggregated docker-ci email report
- Add cleanup to remove leftover containers
* Add nightly release to docker-ci
* Add more tests around auth.ResolveAuthConfig
- Remove a few errors in tests
- Catch errClosing error when TCP and UDP proxies are terminated
* Only run certain tests with TESTFLAGS='-run TestName' make.sh
* Prevent docker-ci to test closing PRs
* Replace panic by log.Fatal in tests
- Increase TestRunDetach timeout

#### Documentation

* Add initial draft of the Docker infrastructure doc
* Add devenvironment link to CONTRIBUTING.md
* Add `apt-get install curl` to Ubuntu docs
* Add explanation for export restrictions
* Add .dockercfg doc
* Remove Gentoo install notes about #1422 workaround
* Fix help text for -v option
* Fix Ping endpoint documentation
- Fix parameter names in docs for ADD command
- Fix ironic typo in changelog
* Various command fixes in postgres example
* Document how to edit and release docs
- Minor updates to `postgresql_service.rst`
* Clarify LGTM process to contributors
- Corrected error in the package name
* Document what `vagrant up` is actually doing
+ improve doc search results
* Cleanup whitespace in API 1.5 docs
* use angle brackets in MAINTAINER example email
* Update archlinux.rst
+ Changes to a new style for the docs. Includes version switcher.
* Formatting, add information about multiline json
* Improve registry and index REST API documentation
- Replace deprecated upgrading reference to docker-latest.tgz, which hasn't been updated since 0.5.3
* Update Gentoo installation documentation now that we're in the portage tree proper
* Cleanup and reorganize docs and tooling for contributors and maintainers
- Minor spelling correction of protocoll -> protocol

#### Contrib

* Add vim syntax highlighting for Dockerfiles from @honza
* Add mkimage-arch.sh
* Reorganize contributed completion scripts to add zsh completion

#### Hack

* Add vagrant user to the docker group
* Add proper bash completion for "docker push"
* Add xz utils as a runtime dep
* Add cleanup/refactor portion of #2010 for hack and Dockerfile updates
+ Add contrib/mkimage-centos.sh back (from #1621), and associated documentation link
* Add several of the small make.sh fixes from #1920, and make the output more consistent and contributor-friendly
+ Add @tianon to hack/MAINTAINERS
* Improve network performance for VirtualBox
* Revamp install.sh to be usable by more people, and to use official install methods whenever possible (apt repo, portage tree, etc.)
- Fix contrib/mkimage-debian.sh apt caching prevention
+ Added Dockerfile.tmLanguage to contrib
* Configured FPM to make /etc/init/docker.conf a config file
* Enable SSH Agent forwarding in Vagrant VM
* Several small tweaks/fixes for contrib/mkimage-debian.sh

#### Other

- Builder: Abort build if mergeConfig returns an error and fix duplicate error message
- Packaging: Remove deprecated packaging directory
- Registry: Use correct auth config when logging in.
- Registry: Fix the error message so it is the same as the regex

## 0.6.3 (2013-09-23)

#### Packaging

* Add 'docker' group on install for ubuntu package
* Update tar vendor dependency
* Download apt key over HTTPS

#### Runtime

- Only copy and change permissions on non-bindmount volumes
* Allow multiple volumes-from
- Fix HTTP imports from STDIN

#### Documentation

* Update section on extracting the docker binary after build
* Update development environment docs for new build process
* Remove 'base' image from documentation

#### Other

- Client: Fix detach issue
- Registry: Update regular expression to match index

## 0.6.2 (2013-09-17)

#### Runtime

+ Add domainname support
+ Implement image filtering with path.Match
* Remove unnecesasry warnings
* Remove os/user dependency
* Only mount the hostname file when the config exists
* Handle signals within the `docker login` command
- UID and GID are now also applied to volumes
- `docker start` set error code upon error
- `docker run` set the same error code as the process started

#### Builder

+ Add -rm option in order to remove intermediate containers
* Allow multiline for the RUN instruction

#### Registry

* Implement login with private registry
- Fix push issues

#### Other

+ Hack: Vendor all dependencies
* Remote API: Bump to v1.5
* Packaging: Break down hack/make.sh into small scripts, one per 'bundle': test, binary, ubuntu etc.
* Documentation: General improvments

## 0.6.1 (2013-08-23)

#### Registry

* Pass "meta" headers in API calls to the registry

#### Packaging

- Use correct upstart script with new build tool
- Use libffi-dev, don`t build it from sources
- Remove duplicate mercurial install command

## 0.6.0 (2013-08-22)

#### Runtime

+ Add lxc-conf flag to allow custom lxc options
+ Add an option to set the working directory
* Add Image name to LogEvent tests
+ Add -privileged flag and relevant tests, docs, and examples
* Add websocket support to /container/<name>/attach/ws
* Add warning when net.ipv4.ip_forwarding = 0
* Add hostname to environment
* Add last stable version in `docker version`
- Fix race conditions in parallel pull
- Fix Graph ByParent() to generate list of child images per parent image.
- Fix typo: fmt.Sprint -> fmt.Sprintf
- Fix small \n error un docker build
* Fix to "Inject dockerinit at /.dockerinit"
* Fix #910. print user name to docker info output
* Use Go 1.1.2 for dockerbuilder
* Use ranged for loop on channels
- Use utils.ParseRepositoryTag instead of strings.Split(name, ":") in server.ImageDelete
- Improve CMD, ENTRYPOINT, and attach docs.
- Improve connect message with socket error
- Load authConfig only when needed and fix useless WARNING
- Show tag used when image is missing
* Apply volumes-from before creating volumes
- Make docker run handle SIGINT/SIGTERM
- Prevent crash when .dockercfg not readable
- Install script should be fetched over https, not http.
* API, issue 1471: Use groups for socket permissions
- Correctly detect IPv4 forwarding
* Mount /dev/shm as a tmpfs
- Switch from http to https for get.docker.io
* Let userland proxy handle container-bound traffic
* Updated the Docker CLI to specify a value for the "Host" header.
- Change network range to avoid conflict with EC2 DNS
- Reduce connect and read timeout when pinging the registry
* Parallel pull
- Handle ip route showing mask-less IP addresses
* Allow ENTRYPOINT without CMD
- Always consider localhost as a domain name when parsing the FQN repos name
* Refactor checksum

#### Documentation

* Add MongoDB image example
* Add instructions for creating and using the docker group
* Add sudo to examples and installation to documentation
* Add ufw doc
* Add a reference to ps -a
* Add information about Docker`s high level tools over LXC.
* Fix typo in docs for docker run -dns
* Fix a typo in the ubuntu installation guide
* Fix to docs regarding adding docker groups
* Update default -H docs
* Update readme with dependencies for building
* Update amazon.rst to explain that Vagrant is not necessary for running Docker on ec2
* PostgreSQL service example in documentation
* Suggest installing linux-headers by default.
* Change the twitter handle
* Clarify Amazon EC2 installation
* 'Base' image is deprecated and should no longer be referenced in the docs.
* Move note about officially supported kernel
- Solved the logo being squished in Safari

#### Builder

+ Add USER instruction do Dockerfile
+ Add workdir support for the Buildfile
* Add no cache for docker build
- Fix docker build and docker events output
- Only count known instructions as build steps
- Make sure ENV instruction within build perform a commit each time
- Forbid certain paths within docker build ADD
- Repository name (and optionally a tag) in build usage
- Make sure ADD will create everything in 0755

#### Remote API

* Sort Images by most recent creation date.
* Reworking opaque requests in registry module
* Add image name in /events
* Use mime pkg to parse Content-Type
* 650 http utils and user agent field

#### Hack

+ Bash Completion: Limit commands to containers of a relevant state
* Add docker dependencies coverage testing into docker-ci

#### Packaging

+ Docker-brew 0.5.2 support and memory footprint reduction
* Add new docker dependencies into docker-ci
- Revert "docker.upstart: avoid spawning a `sh` process"
+ Docker-brew and Docker standard library
+ Release docker with docker
* Fix the upstart script generated by get.docker.io
* Enabled the docs to generate manpages.
* Revert Bind daemon to 0.0.0.0 in Vagrant.

#### Register

* Improve auth push
* Registry unit tests + mock registry

#### Tests

* Improve TestKillDifferentUser to prevent timeout on buildbot
- Fix typo in TestBindMounts (runContainer called without image)
* Improve TestGetContainersTop so it does not rely on sleep
* Relax the lo interface test to allow iface index != 1
* Add registry functional test to docker-ci
* Add some tests in server and utils

#### Other

* Contrib: bash completion script
* Client: Add docker cp command and copy api endpoint to copy container files/folders to the host
* Don`t read from stdout when only attached to stdin

## 0.5.3 (2013-08-13)

#### Runtime

* Use docker group for socket permissions
- Spawn shell within upstart script
- Handle ip route showing mask-less IP addresses
- Add hostname to environment

#### Builder

- Make sure ENV instruction within build perform a commit each time

## 0.5.2 (2013-08-08)

* Builder: Forbid certain paths within docker build ADD
- Runtime: Change network range to avoid conflict with EC2 DNS
* API: Change daemon to listen on unix socket by default

## 0.5.1 (2013-07-30)

#### Runtime

+ Add `ps` args to `docker top`
+ Add support for container ID files (pidfile like)
+ Add container=lxc in default env
+ Support networkless containers with `docker run -n` and `docker -d -b=none`
* Stdout/stderr logs are now stored in the same file as JSON
* Allocate a /16 IP range by default, with fallback to /24. Try 12 ranges instead of 3.
* Change .dockercfg format to json and support multiple auth remote
- Do not override volumes from config
- Fix issue with EXPOSE override

#### API

+ Docker client now sets useragent (RFC 2616)
+ Add /events endpoint

#### Builder

+ ADD command now understands URLs
+ CmdAdd and CmdEnv now respect Dockerfile-set ENV variables
- Create directories with 755 instead of 700 within ADD instruction

#### Hack

* Simplify unit tests with helpers
* Improve docker.upstart event
* Add coverage testing into docker-ci

## 0.5.0 (2013-07-17)

#### Runtime

+ List all processes running inside a container with 'docker top'
+ Host directories can be mounted as volumes with 'docker run -v'
+ Containers can expose public UDP ports (eg, '-p 123/udp')
+ Optionally specify an exact public port (eg. '-p 80:4500')
* 'docker login' supports additional options
- Dont save a container`s hostname when committing an image.

#### Registry

+ New image naming scheme inspired by Go packaging convention allows arbitrary combinations of registries
- Fix issues when uploading images to a private registry

#### Builder

+ ENTRYPOINT instruction sets a default binary entry point to a container
+ VOLUME instruction marks a part of the container as persistent data
* 'docker build' displays the full output of a build by default

## 0.4.8 (2013-07-01)

+ Builder: New build operation ENTRYPOINT adds an executable entry point to the container.  - Runtime: Fix a bug which caused 'docker run -d' to no longer print the container ID.
- Tests: Fix issues in the test suite

## 0.4.7 (2013-06-28)

#### Remote API

* The progress bar updates faster when downloading and uploading large files
- Fix a bug in the optional unix socket transport

#### Runtime

* Improve detection of kernel version
+ Host directories can be mounted as volumes with 'docker run -b'
- fix an issue when only attaching to stdin
* Use 'tar --numeric-owner' to avoid uid mismatch across multiple hosts

#### Hack

* Improve test suite and dev environment
* Remove dependency on unit tests on 'os/user'

#### Other

* Registry: easier push/pull to a custom registry
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

#### Builder

+ ADD of a local file will detect tar archives and unpack them
* ADD improvements: use tar for copy + automatically unpack local archives
* ADD uses tar/untar for copies instead of calling 'cp -ar'
* Fixed the behavior of ADD to be (mostly) reverse-compatible, predictable and well-documented.
- Fix a bug which caused builds to fail if ADD was the first command
* Nicer output for 'docker build'

#### Runtime

* Remove bsdtar dependency
* Add unix socket and multiple -H support
* Prevent rm of running containers
* Use go1.1 cookiejar
- Fix issue detaching from running TTY container
- Forbid parralel push/pull for a single image/repo. Fixes #311
- Fix race condition within Run command when attaching.

#### Client

* HumanReadable ProgressBar sizes in pull
* Fix docker version`s git commit output

#### API

* Send all tags on History API call
* Add tag lookup to history command. Fixes #882

#### Documentation

- Fix missing command in irc bouncer example

## 0.4.2 (2013-06-17)

- Packaging: Bumped version to work around an Ubuntu bug

## 0.4.1 (2013-06-17)

#### Remote Api

+ Add flag to enable cross domain requests
+ Add images and containers sizes in docker ps and docker images

#### Runtime

+ Configure dns configuration host-wide with 'docker -d -dns'
+ Detect faulty DNS configuration and replace it with a public default
+ Allow docker run <name>:<id>
+ You can now specify public port (ex: -p 80:4500)
* Improved image removal to garbage-collect unreferenced parents

#### Client

* Allow multiple params in inspect
* Print the container id before the hijack in `docker run`

#### Registry

* Add regexp check on repo`s name
* Move auth to the client
- Remove login check on pull

#### Other

* Vagrantfile: Add the rest api port to vagrantfile`s port_forward
* Upgrade to Go 1.1
- Builder: don`t ignore last line in Dockerfile when it doesn`t end with \n

## 0.4.0 (2013-06-03)

#### Builder

+ Introducing Builder
+ 'docker build' builds a container, layer by layer, from a source repository containing a Dockerfile

#### Remote API

+ Introducing Remote API
+ control Docker programmatically using a simple HTTP/json API

#### Runtime

* Various reliability and usability improvements

## 0.3.4 (2013-05-30)

#### Builder

+ 'docker build' builds a container, layer by layer, from a source repository containing a Dockerfile
+ 'docker build -t FOO' applies the tag FOO to the newly built container.

#### Runtime

+ Interactive TTYs correctly handle window resize
* Fix how configuration is merged between layers

#### Remote API

+ Split stdout and stderr on 'docker run'
+ Optionally listen on a different IP and port (use at your own risk)

#### Documentation

* Improved install instructions.

## 0.3.3 (2013-05-23)

- Registry: Fix push regression
- Various bugfixes

## 0.3.2 (2013-05-09)

#### Registry

* Improve the checksum process
* Use the size to have a good progress bar while pushing
* Use the actual archive if it exists in order to speed up the push
- Fix error 400 on push

#### Runtime

* Store the actual archive on commit

## 0.3.1 (2013-05-08)

#### Builder

+ Implement the autorun capability within docker builder
+ Add caching to docker builder
+ Add support for docker builder with native API as top level command
+ Implement ENV within docker builder
- Check the command existance prior create and add Unit tests for the case
* use any whitespaces instead of tabs

#### Runtime

+ Add go version to debug infos
* Kernel version - don`t show the dash if flavor is empty

#### Registry

+ Add docker search top level command in order to search a repository
- Fix pull for official images with specific tag
- Fix issue when login in with a different user and trying to push
* Improve checksum - async calculation

#### Images

+ Output graph of images to dot (graphviz)
- Fix ByParent function

#### Documentation

+ New introduction and high-level overview
+ Add the documentation for docker builder
- CSS fix for docker documentation to make REST API docs look better.
- Fix CouchDB example page header mistake
- Fix README formatting
* Update www.docker.io website.

#### Other

+ Website: new high-level overview
- Makefile: Swap "go get" for "go get -d", especially to compile on go1.1rc
* Packaging: packaging ubuntu; issue #510: Use goland-stable PPA package to build docker

## 0.3.0 (2013-05-06)

#### Runtime

- Fix the command existance check
- strings.Split may return an empty string on no match
- Fix an index out of range crash if cgroup memory is not

#### Documentation

* Various improvments
+ New example: sharing data between 2 couchdb databases

#### Other

* Vagrant: Use only one deb line in /etc/apt
+ Registry: Implement the new registry

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

Initial public release

- Implement registry in order to push/pull images
- TCP port allocation
- Fix termcaps on Linux
- Add documentation
- Add Vagrant support with Vagrantfile
- Add unit tests
- Add repository/tags to ease image management
- Improve the layer implementation
