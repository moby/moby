#!/usr/bin/env bash

install_binary() {
	local file="$1"
	local target="${DOCKER_MAKE_INSTALL_PREFIX:=/usr/local}/bin/"
	if [ "$(go env GOOS)" == "linux" ]; then
		echo "Installing $(basename $file) to ${target}"
		mkdir -p "$target"
		cp -f -L "$file" "$target"
	else
		echo "Install is only supported on linux"
		return 1
	fi
}
