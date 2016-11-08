#!/bin/sh
set -e
set -x

TOMLV_COMMIT=9baf8a8a9f2ed20a8e54160840c492f937eeaf9a
RUNC_COMMIT=ac031b5bf1cc92239461125f4c1ffb760522bbf2
CONTAINERD_COMMIT=8517738ba4b82aff5662c97ca4627e7e4d03b531
GRIMES_COMMIT=fe069a03affd2547fdb05e5b8b07202d2e41735b
LIBNETWORK_COMMIT=0f534354b813003a754606689722fe253101bc4e
VNDR_COMMIT=f56bd4504b4fad07a357913687fb652ee54bb3b0

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
	git clone https://github.com/docker/runc.git "$GOPATH/src/github.com/opencontainers/runc"
	cd "$GOPATH/src/github.com/opencontainers/runc"
	git checkout -q "$RUNC_COMMIT"
	make BUILDTAGS="$RUNC_BUILDTAGS" $1
	cp runc /usr/local/bin/docker-runc
}

install_containerd() {
	echo "Install containerd version $CONTAINERD_COMMIT"
	git clone https://github.com/docker/containerd.git "$GOPATH/src/github.com/docker/containerd"
	cd "$GOPATH/src/github.com/docker/containerd"
	git checkout -q "$CONTAINERD_COMMIT"
	make $1
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
			install_containerd static
			;;

		containerd-dynamic)
			install_containerd
			;;

		grimes)
			echo "Install grimes version $GRIMES_COMMIT"
			git clone https://github.com/crosbymichael/grimes.git "$GOPATH/grimes"
			cd "$GOPATH/grimes"
			git checkout -q "$GRIMES_COMMIT"
			make
			cp init /usr/local/bin/docker-init
			;;

		proxy)
			export CGO_ENABLED=0
			install_proxy
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

		*)
			echo echo "Usage: $0 [tomlv|runc|containerd|grimes|proxy]"
			exit 1

	esac
done

if [ $RM_GOPATH -eq 1 ]; then
	rm -rf "$GOPATH"
fi
