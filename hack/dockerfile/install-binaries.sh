#!/bin/sh
set -e
set -x

TOMLV_COMMIT=9baf8a8a9f2ed20a8e54160840c492f937eeaf9a
RUNC_COMMIT=cc29e3dded8e27ba8f65738f40d251c885030a28
CONTAINERD_COMMIT=2545227b0357eb55e369fa0072baef9ad91cdb69
GRIMES_COMMIT=f207601a8d19a534cc90d9e26e037e9931ccb9db

export GOPATH="$(mktemp -d)"

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
			echo "Install runc version $RUNC_COMMIT"
			git clone https://github.com/opencontainers/runc.git "$GOPATH/src/github.com/opencontainers/runc"
			cd "$GOPATH/src/github.com/opencontainers/runc"
			git checkout -q "$RUNC_COMMIT"
			make static BUILDTAGS="seccomp apparmor selinux"
			cp runc /usr/local/bin/docker-runc
			;;

		containerd)
			echo "Install containerd version $CONTAINERD_COMMIT"
			git clone https://github.com/docker/containerd.git "$GOPATH/src/github.com/docker/containerd"
			cd "$GOPATH/src/github.com/docker/containerd"
			git checkout -q "$CONTAINERD_COMMIT"
			make static
			cp bin/containerd /usr/local/bin/docker-containerd
			cp bin/containerd-shim /usr/local/bin/docker-containerd-shim
			cp bin/ctr /usr/local/bin/docker-containerd-ctr
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
