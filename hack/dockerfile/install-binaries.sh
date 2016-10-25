#!/bin/sh
set -e
set -x

TOMLV_COMMIT=9baf8a8a9f2ed20a8e54160840c492f937eeaf9a
RUNC_COMMIT=02f8fa7863dd3f82909a73e2061897828460d52f
CONTAINERD_COMMIT=52ef1ceb4b660c42cf4ea9013180a5663968d4c7
GRIMES_COMMIT=74341e923bdf06cfb6b70cf54089c4d3ac87ec2d

export GOPATH="$(mktemp -d)"

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
	git clone https://github.com/docker/containerd.git "$GOPATH/src/github.com/docker/containerd"
	cd "$GOPATH/src/github.com/docker/containerd"
	git checkout -q "$CONTAINERD_COMMIT"
	make $1
	cp bin/containerd /usr/local/bin/docker-containerd
	cp bin/containerd-shim /usr/local/bin/docker-containerd-shim
	cp bin/ctr /usr/local/bin/docker-containerd-ctr
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

		*)
			echo echo "Usage: $0 [tomlv|runc|containerd|grimes]"
			exit 1

	esac
done

rm -rf "$GOPATH"
