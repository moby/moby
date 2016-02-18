# Jenkins CI script for Windows to Linux CI.
# Heavily modified by John Howard (@jhowardmsft) December 2015 to try to make it more reliable.
set +x
set +e
SCRIPT_VER="18-Feb-2016 11:47 PST"

# TODO to make (even) more resilient: 
#  - Check if jq is installed
#  - Make sure bash is v4.3 or later. Can't do until all Azure nodes on the latest version
#  - Make sure we are not running as local system. Can't do until all Azure nodes are updated.
#  - Error if docker versions are not equal. Can't do until all Azure nodes are updated
#  - Error if go versions are not equal. Can't do until all Azure nodes are updated.
#  - Error if running 32-bit posix tools. Probably can take from bash --version and check contains "x86_64"
#  - Warn if the CI directory cannot be deleted afterwards. Otherwise turdlets are left behind
#  - Use %systemdrive% ($SYSTEMDRIVE) rather than hard code to c: for TEMP
#  - Consider cross builing the Windows binary and copy across. That's a bit of a heavy lift. Only reason
#    for doing that is that it mirrors the actual release process for docker.exe which is cross-built.
#    However, should absolutely not be a problem if built natively, so nit-picking.
#  - Tidy up of images and containers. Either here, or in the teardown script.

ec=0
uniques=1
echo INFO: Started at `date`. Script version $SCRIPT_VER

# get the ip
ip="${DOCKER_HOST#*://}"
ip="${ip%%:*}"

# make sure it is the right DOCKER_HOST. No, this is not a typo, it really
# is at port 2357. This is the daemon which is running on the Linux host.
# The way CI works is to launch a second daemon, docker-in-docker, which
# listens on port 2375 and is built from sources matching the PR. That's the
# one which is tested against.
export DOCKER_HOST="tcp://$ip:2357"

# Save for use by make.sh and scripts it invokes
export MAIN_DOCKER_HOST="$DOCKER_HOST"


# Verify we can get the remote node to respond to _ping
if [ $ec -eq 0 ]; then
	reply=`curl -s http://$ip:2357/_ping`
	if [ "$reply" != "OK" ]; then
		ec=1
		echo "ERROR: Failed to get OK response from Linux node at $ip:2357. It may be down."
		echo "       Try re-running this CI job, or ask on #docker-dev or #docker-maintainers"
		echo "       to see if the node is up and running."
		echo
		echo "Regular ping output for remote host below. It should reply. If not, it needs restarting."
		ping $ip
	else
		echo "INFO: The Linux nodes outer daemon replied to a ping. Good!"
	fi 
fi

# Get the version from the remote node. Note this may fail if jq is not installed.
# That's probably worth checking to make sure, just in case.
if [ $ec -eq 0 ]; then
	remoteVersion=`curl -s http://$ip:2357/version | jq -c '.Version'`
	echo "INFO: Remote daemon is running docker version $remoteVersion"
fi

# Compare versions. We should really fail if result is no 1. Output at end of script.
if [ $ec -eq 0 ]; then
	uniques=`docker version | grep Version | /usr/bin/sort -u | wc -l`
fi

# Make sure we are in repo
if [ $ec -eq 0 ]; then
	if [ ! -d hack ]; then
		echo "ERROR: Are you sure this is being launched from a the root of docker repository?"
		echo "       If this is a Windows CI machine, it should be c:\jenkins\gopath\src\github.com\docker\docker."
                echo "       Current directory is `pwd`"
		ec=1
	fi
fi

# Get the commit has and verify we have something
if [ $ec -eq 0 ]; then
	export COMMITHASH=$(git rev-parse --short HEAD)
	echo INFO: Commmit hash is $COMMITHASH
	if [ -z $COMMITHASH ]; then
		echo "ERROR: Failed to get commit hash. Are you sure this is a docker repository?"
		ec=1
	fi
fi

# Redirect to a temporary location. Check is here for local runs from Jenkins machines just in case not
# in the right directory where the repo is cloned. We also redirect TEMP to not use the environment
# TEMP as when running as a standard user (not local system), it otherwise exposes a bug in posix tar which
# will cause CI to fail from Windows to Linux. Obviously it's not best practice to ever run as local system...
if [ $ec -eq 0 ]; then
	export TEMP=/c/CI/CI-$COMMITHASH
	export TMP=$TMP
	/usr/bin/mkdir -p $TEMP  # Make sure Linux mkdir for -p
fi

# Tidy up time
if [ $ec -eq 0 ]; then
	echo INFO: Deleting pre-existing containers and images...
	# Force remove all containers based on a previously built image with this commit
	! docker rm -f $(docker ps -aq --filter "ancestor=docker:$COMMITHASH") &>/dev/null

	# Force remove any container with this commithash as a name
	! docker rm -f $(docker ps -aq --filter "name=docker-$COMMITHASH") &>/dev/null

	# Force remove the image if it exists
	! docker rmi -f "docker-$COMMITHASH" &>/dev/null
	
	# This SHOULD never happen, but just in case, also blow away any containers
	# that might be around. 
	! if [ ! `docker ps -aq | wc -l` -eq 0 ]; then
		echo WARN: There were some leftover containers. Cleaning them up.
		! docker rm -f $(docker ps -aq)
	fi
fi

# Provide the docker version for debugging purposes. If these fail, game over. 
# as the Linux box isn't responding for some reason.
if [ $ec -eq 0 ]; then
	echo INFO: Docker version and info of the outer daemon on the Linux node
	echo
	docker version
	ec=$?
	if [ 0 -ne $ec ]; then
		echo "ERROR: The main linux daemon does not appear to be running. Has the Linux node crashed?"
	fi
	echo
fi

# Same as above, but docker info
if [ $ec -eq 0 ]; then
	echo
	docker info
	ec=$?
	if [ 0 -ne $ec ]; then
		echo "ERROR: The main linux daemon does not appear to be running. Has the Linux node crashed?"
	fi
	echo
fi

# build the daemon image
if [ $ec -eq 0 ]; then
	echo "INFO: Running docker build on Linux host at $DOCKER_HOST"
	set -x
	docker build --rm --force-rm -t "docker:$COMMITHASH" .
	ec=$?
	set +x
	if [ 0 -ne $ec ]; then
		echo "ERROR: docker build failed"
	fi
fi

# Start the docker-in-docker daemon from the image we just built
if [ $ec -eq 0 ]; then
	echo "INFO: Starting build of a Linux daemon to test against, and starting it..."
	set -x
	docker run --pid host --privileged -d --name "docker-$COMMITHASH" --net host "docker:$COMMITHASH" bash -c 'echo "INFO: Compiling" && date && hack/make.sh binary && echo "INFO: Compile complete" && date && cp bundles/$(cat VERSION)/binary/docker /bin/docker && echo "INFO: Starting daemon" && exec docker daemon -D -H tcp://0.0.0.0:2375'
	ec=$?
	set +x
	if [ 0 -ne $ec ]; then
	    	echo "ERROR: Failed to compile and start the linux daemon"
	fi
fi

# Build locally.
if [ $ec -eq 0 ]; then
	echo "INFO: Starting local build of Windows binary..."
	set -x
	export TIMEOUT="120m"
	export DOCKER_HOST="tcp://$ip:2375"
	export DOCKER_TEST_HOST="tcp://$ip:2375"
	unset DOCKER_CLIENTONLY
	export DOCKER_REMOTE_DAEMON=1
	hack/make.sh binary 
	ec=$?
	set +x
	if [ 0 -ne $ec ]; then
	    echo "ERROR: Build of binary on Windows failed"
	fi
fi

# Make a local copy of the built binary and ensure that is first in our path
if [ $ec -eq 0 ]; then
	VERSION=$(< ./VERSION)
	cp bundles/$VERSION/binary/docker.exe $TEMP
	ec=$?
	if [ 0 -ne $ec ]; then
		echo "ERROR: Failed to copy built binary to $TEMP"
	fi
	export PATH=$TEMP:$PATH
fi

# Run the integration tests
if [ $ec -eq 0 ]; then
	echo "INFO: Running Integration tests..."
	set -x
	hack/make.sh test-integration-cli
	ec=$?
	set +x
	if [ 0 -ne $ec ]; then
		echo "ERROR: CLI test failed."
		# Next line is useful, but very long winded if included
		# docker -H=$MAIN_DOCKER_HOST logs "docker-$COMMITHASH"
	fi
fi

# Tidy up any temporary files from the CI run
if [ ! -z $COMMITHASH ]; then
	rm -rf $TEMP
fi

# CI Integrity check - ensure we are using the same version of go as present in the Dockerfile
GOVER_DOCKERFILE=`grep 'ENV GO_VERSION' Dockerfile | awk '{print $3}'`
GOVER_INSTALLED=`go version | awk '{print $3}'`
if [ "${GOVER_INSTALLED:2}" != "$GOVER_DOCKERFILE" ]; then
	#ec=1  # Uncomment to make CI fail once all nodes are updated.
	echo
	echo "---------------------------------------------------------------------------"
	echo "WARN: CI should be using go version $GOVER_DOCKERFILE, but is using ${GOVER_INSTALLED:2}"
	echo "      Please ping #docker-maintainers on IRC to get this CI server updated."
	echo "---------------------------------------------------------------------------"
	echo
fi

# Check the Linux box is running a matching version of docker
if [ "$uniques" -ne 1 ]; then
    ec=0  # Uncomment to make CI fail once all nodes are updated.
	echo
	echo "---------------------------------------------------------------------------"
	echo "ERROR: This CI node is not running the same version of docker as the daemon."
	echo "       This is a CI configuration issue."
	echo "---------------------------------------------------------------------------"
	echo
fi

# Tell the user how we did.
if [ $ec -eq 0 ]; then
	echo INFO: Completed successfully at `date`. 
else
	echo ERROR: Failed with exitcode $ec at `date`.
fi
exit $ec
