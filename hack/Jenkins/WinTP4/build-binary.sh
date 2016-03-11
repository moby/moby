
# Build the binary in a container
if [ $ec -eq 0 ]; then
    echo "INFO: Building the test binary..."
    set -x
    docker run --rm -v "$TEMPWIN:c:\target" docker \
        sh -c 'cd /c/go/src/github.com/docker/docker; \
            hack/make.sh binary; \
            ec=$?; \
            if [ $ec -eq 0 ]; then \
                robocopy /c/go/src/github.com/docker/docker/bundles/$(cat VERSION)/binary /c/target/binary; \
            fi; \
            exit $ec'
    ec=$?
    set +x
    if [ 0 -ne $ec ]; then
        echo
        echo "----------------------------------"
        echo "ERROR: Failed to build test binary"
        echo "----------------------------------"
        echo
    fi
fi

# Copy the built docker.exe to docker-$COMMITHASH.exe so that it is
# easily spotted in task manager, and to make sure the built binaries are first
# on our path
if [ $ec -eq 0 ]; then
    echo "INFO: Linking the built binary to $TEMP/docker-$COMMITHASH.exe..."
    ln $TEMP/binary/docker.exe $TEMP/binary/docker-$COMMITHASH.exe
    ec=$?
    if [ 0 -ne $ec ]; then
        echo "ERROR: Failed to link"
    fi
fi
