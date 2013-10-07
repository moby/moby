# Docker packaging

This directory has one subdirectory per packaging distribution.
At minimum, each of these subdirectories should contain a
README.$DISTRIBUTION explaining how to create the native
docker package and how to install it.

**Important:** the debian and ubuntu directories are here for
reference only. Since we experienced many issues with Launchpad,
we gave up on using it to have a Docker PPA (at least, for now!)
and we are using a simpler process.
See [/hack/release](../hack/release) for details.
