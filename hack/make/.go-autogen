#!/usr/bin/env bash

rm -rf autogen/*

source hack/dockerfile/install/runc.installer
source hack/dockerfile/install/tini.installer
source hack/dockerfile/install/containerd.installer

LDFLAGS="${LDFLAGS} \
	-X \"github.com/docker/docker/dockerversion.Version=${VERSION}\" \
	-X \"github.com/docker/docker/dockerversion.GitCommit=${GITCOMMIT}\" \
	-X \"github.com/docker/docker/dockerversion.BuildTime=${BUILDTIME}\" \
	-X \"github.com/docker/docker/dockerversion.IAmStatic=${IAMSTATIC:-true}\" \
	-X \"github.com/docker/docker/dockerversion.PlatformName=${PLATFORM}\" \
	-X \"github.com/docker/docker/dockerversion.ProductName=${PRODUCT}\" \
	-X \"github.com/docker/docker/dockerversion.DefaultProductLicense=${DEFAULT_PRODUCT_LICENSE}\" \
"

# Compile the Windows resources into the sources
if [ "$(go env GOOS)" = "windows" ]; then
	mkdir -p autogen/winresources/tmp autogen/winresources/dockerd
	cp hack/make/.resources-windows/resources.go autogen/winresources/dockerd/

	if [ "$(go env GOHOSTOS)" == "windows" ]; then
		WINDRES=windres
		WINDMC=windmc
	else
		# Cross compiling
		WINDRES=x86_64-w64-mingw32-windres
		WINDMC=x86_64-w64-mingw32-windmc
	fi

	# Generate a Windows file version of the form major,minor,patch,build (with any part optional)
	if [ ! -v VERSION_QUAD ]; then
		VERSION_QUAD=$(echo -n $VERSION | sed -re 's/^([0-9.]*).*$/\1/' | tr . ,)
	fi

	# Pass version and commit information into the resource compiler
	defs=
	[ ! -z $VERSION ]      && defs="$defs -D DOCKER_VERSION=\"$VERSION\""
	[ ! -z $VERSION_QUAD ] && defs="$defs -D DOCKER_VERSION_QUAD=$VERSION_QUAD"
	[ ! -z $GITCOMMIT ]    && defs="$defs -D DOCKER_COMMIT=\"$GITCOMMIT\""

	function makeres {
		${WINDRES} \
			-i hack/make/.resources-windows/$1 \
			-o $3 \
			-F $2 \
			--use-temp-file \
			-I autogen/winresources/tmp \
			$defs
	}

	${WINDMC} \
		hack/make/.resources-windows/event_messages.mc \
		-h autogen/winresources/tmp \
		-r autogen/winresources/tmp

	makeres dockerd.rc pe-x86-64 autogen/winresources/dockerd/rsrc_amd64.syso

	rm -r autogen/winresources/tmp
fi
