#!/usr/bin/env bash

set -e
rm -rf "$DEST"

if ! command -v dockerd &> /dev/null; then
	echo >&2 'error: binary-daemon or dynbinary-daemon must be run before run'
	false
fi

DOCKER_GRAPHDRIVER=${DOCKER_GRAPHDRIVER:-vfs}
DOCKER_USERLANDPROXY=${DOCKER_USERLANDPROXY:-true}

# example usage: DOCKER_STORAGE_OPTS="dm.basesize=20G,dm.loopdatasize=200G"
storage_params=""
if [ -n "$DOCKER_STORAGE_OPTS" ]; then
	IFS=','
	for i in ${DOCKER_STORAGE_OPTS}; do
		storage_params="--storage-opt $i $storage_params"
	done
	unset IFS
fi

listen_port=2375
if [ -n "$DOCKER_PORT" ]; then
	IFS=':' read -r -a ports <<< "$DOCKER_PORT"
	listen_port="${ports[-1]}"
fi

extra_params="$DOCKERD_ARGS"
if [ "$DOCKER_REMAP_ROOT" ]; then
	extra_params="$extra_params --userns-remap $DOCKER_REMAP_ROOT"
fi

if [ -n "$DOCKER_EXPERIMENTAL" ]; then
	extra_params="$extra_params --experimental"
fi

dockerd="dockerd"
socket=/var/run/docker.sock
if [ -n "$DOCKER_ROOTLESS" ]; then
	user="unprivilegeduser"
	uid=$(id -u $user)
	# shellcheck disable=SC2174
	mkdir -p -m 700 "/tmp/docker-${uid}"
	chown $user "/tmp/docker-${uid}"
	dockerd="sudo -u $user -E XDG_RUNTIME_DIR=/tmp/docker-${uid} -E HOME=/home/${user} -- dockerd-rootless.sh"
	socket=/tmp/docker-${uid}/docker.sock
fi

args="--debug \
	--host "tcp://0.0.0.0:${listen_port}" --host "unix://${socket}" \
	--storage-driver "${DOCKER_GRAPHDRIVER}" \
	--userland-proxy="${DOCKER_USERLANDPROXY}" \
	$storage_params \
	$extra_params"

echo "${dockerd} ${args}"
# shellcheck disable=SC2086
exec "${dockerd}" ${args}
