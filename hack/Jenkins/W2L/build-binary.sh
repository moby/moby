
# Build locally.
if [ $ec -eq 0 ]; then
	echo "INFO: Starting local build of Windows binary..."
	set -x
	export TIMEOUT="120m"
	export DOCKER_HOST="tcp://$ip:$port_inner"
	export DOCKER_TEST_HOST="tcp://$ip:$port_inner"
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
