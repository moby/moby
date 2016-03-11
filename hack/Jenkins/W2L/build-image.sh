
# build the daemon image
if [ $ec -eq 0 ]; then
	echo "INFO: Running docker build on Linux host at $DOCKER_HOST"
	set -x
	docker build --rm --force-rm -t "docker:$COMMITHASH" .
    cat <<EOF | docker build --rm --force-rm -t "docker:$COMMITHASH" -
FROM docker:$COMMITHASH
RUN hack/make.sh binary
RUN cp bundles/latest/binary/docker /bin/docker
CMD docker daemon -D -H tcp://0.0.0.0:$port_inner $daemon_extra_args
EOF
	ec=$?
	set +x
	if [ 0 -ne $ec ]; then
		echo "ERROR: docker build failed"
	fi
fi
