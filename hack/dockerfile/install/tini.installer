#!/bin/sh

# TINI_VERSION specifies the version of tini (docker-init) to build, and install
# from the https://github.com/krallin/tini repository. This binary is used
# when starting containers with the `--init` option.
: "${TINI_VERSION:=v0.19.0}"

install_tini() {
	echo "Install tini version $TINI_VERSION"
	git clone https://github.com/krallin/tini.git "$GOPATH/tini"
	cd "$GOPATH/tini"
	git checkout -q "$TINI_VERSION"
	cmake .
	make tini-static
	mkdir -p "${PREFIX}"
	cp tini-static "${PREFIX}/docker-init"
}
