#!/usr/bin/env bash
set -e
set -x

. $(dirname "$0")/binaries-commits

RM_GOPATH=0

TMP_GOPATH=${TMP_GOPATH:-""}

if [ -z "$TMP_GOPATH" ]; then
	export GOPATH="$(mktemp -d)"
	RM_GOPATH=1
else
	export GOPATH="$TMP_GOPATH"
fi

# Do not build with ambient capabilities support
RUNC_BUILDTAGS="${RUNC_BUILDTAGS:-"seccomp apparmor selinux"}"

install_runc() {
	echo "Install runc version $RUNC_COMMIT"
	git clone https://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc"
	cd "$GOPATH/src/github.com/opencontainers/runc"
	git checkout -q "$RUNC_COMMIT"
	make BUILDTAGS="$RUNC_BUILDTAGS" $1
	cp runc /usr/local/bin/docker-runc
}

install_containerd() {
	echo "Install containerd version $CONTAINERD_COMMIT"
	git clone https://github.com/containerd/containerd.git "$GOPATH/src/github.com/containerd/containerd"
	cd "$GOPATH/src/github.com/containerd/containerd"
	git checkout -q "$CONTAINERD_COMMIT"
	(
		export GOPATH
		make
	)
	cp bin/containerd /usr/local/bin/docker-containerd
	cp bin/containerd-shim /usr/local/bin/docker-containerd-shim
	cp bin/ctr /usr/local/bin/docker-containerd-ctr
}

install_containerd_static() {
	echo "Install containerd version $CONTAINERD_COMMIT"
	git clone https://github.com/containerd/containerd.git "$GOPATH/src/github.com/containerd/containerd"
	cd "$GOPATH/src/github.com/containerd/containerd"
	git checkout -q "$CONTAINERD_COMMIT"
	(
		export GOPATH
		make EXTRA_FLAGS="-buildmode pie" EXTRA_LDFLAGS="-extldflags \\\"-fno-PIC -static\\\""
	)
	cp bin/containerd /usr/local/bin/docker-containerd
	cp bin/containerd-shim /usr/local/bin/docker-containerd-shim
	cp bin/ctr /usr/local/bin/docker-containerd-ctr
}

install_proxy() {
	echo "Install docker-proxy version $LIBNETWORK_COMMIT"
	git clone https://github.com/docker/libnetwork.git "$GOPATH/src/github.com/docker/libnetwork"
	cd "$GOPATH/src/github.com/docker/libnetwork"
	git checkout -q "$LIBNETWORK_COMMIT"
	go build -ldflags="$PROXY_LDFLAGS" -o /usr/local/bin/docker-proxy github.com/docker/libnetwork/cmd/proxy
}

install_dockercli() {
	DOCKERCLI_CHANNEL=${DOCKERCLI_CHANNEL:-edge}
	DOCKERCLI_VERSION=${DOCKERCLI_VERSION:-17.06.0-ce}
	echo "Install docker/cli version $DOCKERCLI_VERSION from $DOCKERCLI_CHANNEL"

	arch=$(uname -m)
	# No official release of these platforms
	if [[ "$arch" != "x86_64" ]] && [[ "$arch" != "s390x" ]]; then
		build_dockercli
		return
	fi

	url=https://download.docker.com/linux/static
	curl -Ls $url/$DOCKERCLI_CHANNEL/$arch/docker-$DOCKERCLI_VERSION.tgz | \
	tar -xz docker/docker
	mv docker/docker /usr/local/bin/
	rmdir docker
}

build_dockercli() {
	DOCKERCLI_VERSION=${DOCKERCLI_VERSION:-17.06.0-ce}
	git clone https://github.com/docker/docker-ce "$GOPATH/tmp/docker-ce"
	cd "$GOPATH/tmp/docker-ce"
	git checkout -q "v$DOCKERCLI_VERSION"
	mkdir -p "$GOPATH/src/github.com/docker"
	mv components/cli "$GOPATH/src/github.com/docker/cli"
	go build -o /usr/local/bin/docker github.com/docker/cli/cmd/docker
}

install_gometalinter() {
	echo "Installing gometalinter version $GOMETALINTER_COMMIT"
	go get -d github.com/alecthomas/gometalinter
	cd "$GOPATH/src/github.com/alecthomas/gometalinter"
	git checkout -q "$GOMETALINTER_COMMIT"
	go build -o /usr/local/bin/gometalinter github.com/alecthomas/gometalinter
	GOBIN=/usr/local/bin gometalinter --install
}

for prog in "$@"
do
	case $prog in
		tomlv)
			echo "Install tomlv version $TOMLV_COMMIT"
			git clone https://github.com/BurntSushi/toml.git "$GOPATH/src/github.com/BurntSushi/toml"
			cd "$GOPATH/src/github.com/BurntSushi/toml" && git checkout -q "$TOMLV_COMMIT"
			go build -v -o /usr/local/bin/tomlv github.com/BurntSushi/toml/cmd/tomlv
			;;

		runc)
			install_runc static
			;;

		runc-dynamic)
			install_runc
			;;

		containerd)
			install_containerd_static
			;;

		containerd-dynamic)
			install_containerd
			;;

		gometalinter)
			install_gometalinter
			;;

		tini)
			echo "Install tini version $TINI_COMMIT"
			git clone https://github.com/krallin/tini.git "$GOPATH/tini"
			cd "$GOPATH/tini"
			git checkout -q "$TINI_COMMIT"
			cmake .
			make tini-static
			cp tini-static /usr/local/bin/docker-init
			;;

		proxy)
			(
				export CGO_ENABLED=0
				install_proxy
			)
			;;

		proxy-dynamic)
			PROXY_LDFLAGS="-linkmode=external" install_proxy
			;;

		vndr)
			echo "Install vndr version $VNDR_COMMIT"
			git clone https://github.com/LK4D4/vndr.git "$GOPATH/src/github.com/LK4D4/vndr"
			cd "$GOPATH/src/github.com/LK4D4/vndr"
			git checkout -q "$VNDR_COMMIT"
			go build -v -o /usr/local/bin/vndr .
			;;

		dockercli)
			install_dockercli
			;;

		*)
			echo echo "Usage: $0 [tomlv|runc|runc-dynamic|containerd|containerd-dynamic|tini|proxy|proxy-dynamic|vndr|dockercli|gometalinter]"
			exit 1

	esac
done

if [ $RM_GOPATH -eq 1 ]; then
	rm -rf "$GOPATH"
fi
