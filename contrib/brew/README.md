# docker-brew

docker-brew is a command-line tool used to build the docker standard library.

## Install instructions

1. Install python if it isn't already available on your OS of choice
1. Install the easy_install tool (`sudo apt-get install python-setuptools`
for Debian)
1. Install the python package manager, `pip` (`easy_install pip`)
1. Run the following command: `pip install -r requirements.txt`
1. You should now be able to use the `docker-brew` script as such.

## Basics

	./docker-brew -h

Display usage and help.

	./docker-brew

Default build from the default repo/branch. Images will be created under the
`library/` namespace. Does not perform a remote push.

	./docker-brew -n mycorp.com -b stable --push git://github.com/mycorp/docker

Will fetch the library definition files in the `stable` branch of the
`git://github.com/mycorp/docker` repository and create images under the
`mycorp.com` namespace (e.g. `mycorp.com/ubuntu`). Created images will then
be pushed to the official docker repository (pending: support for private
repositories)

## Library definition files

The library definition files are plain text files found in the `library/`
subfolder of the docker repository.

### File names

The name of a definition file will determine the name of the image(s) it
creates. For example, the `library/ubuntu` file will create images in the
`<namespace>/ubuntu` repository. If multiple instructions are present in
a single file, all images are expected to be created under a different tag.

### Instruction format

Each line represents a build instruction.
There are different formats that `docker-brew` is able to parse.

	<git-url>
	git://github.com/dotcloud/hipache
	https://github.com/dotcloud/docker.git

The simplest format. `docker-brew` will fetch data from the provided git
repository from the `HEAD`of its `master` branch. Generated image will be
tagged as `latest`. Use of this format is discouraged because there is no
way to ensure stability.

	<docker-tag> <git-url>
	bleeding-edge git://github.com/dotcloud/docker
	unstable https://github.com/dotcloud/docker-redis.git

A more advanced format. `docker-brew` will fetch data from the provided git
repository from the `HEAD`of its `master` branch. Generated image will be
tagged as `<docker-tag>`. Recommended if we always want to provide a snapshot
of the latest development. Again, no way to ensure stability.

	<docker-tag>	<git-url>	T:<git-tag>
	2.4.0 	git://github.com/dotcloud/docker-redis	T:2.4.0
	<docker-tag>	<git-url>	B:<git-branch>
	zfs		git://github.com/dotcloud/docker	B:zfs-support
	<docker-tag>	<git-url>	C:<git-commit-id>
	2.2.0 	https://github.com/dotcloud/docker-redis.git C:a4bf8923ee4ec566d3ddc212

The most complete format. `docker-brew` will fetch data from the provided git
repository from the provided reference (if it's a branch, brew will fetch its
`HEAD`). Generated image will be tagged as `<docker-tag>`. Recommended whenever
possible.