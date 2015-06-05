#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."

# Downloads dependencies into vendor/ directory
mkdir -p vendor

clone() {
	vcs=$1
	pkg=$2
	rev=$3

	pkg_url=https://$pkg
	target_dir=vendor/src/$pkg

	echo -n "$pkg @ $rev: "

	if [ -d $target_dir ]; then
		echo -n 'rm old, '
		rm -fr $target_dir
	fi

	echo -n 'clone, '
	case $vcs in
		git)
			git clone --quiet --no-checkout $pkg_url $target_dir
			( cd $target_dir && git reset --quiet --hard $rev )
			;;
		hg)
			hg clone --quiet --updaterev $rev $pkg_url $target_dir
			;;
	esac

	echo done
}

# the following lines are in sorted order, FYI
clone git github.com/Sirupsen/logrus v0.8.2 # logrus is a common dependency among multiple deps
clone git github.com/docker/libtrust 230dfd18c232
clone git github.com/go-check/check 64131543e7896d5bcc6bd5a76287eb75ea96c673
clone git github.com/gorilla/context 14f550f51a
clone git github.com/gorilla/mux e444e69cbd
clone git github.com/kr/pty 5cf931ef8f
clone git github.com/mistifyio/go-zfs v2.1.1
clone git github.com/tchap/go-patricia v2.1.0
clone hg code.google.com/p/go.net 84a4013f96e0
clone hg code.google.com/p/gosqlite 74691fb6f837

#get libnetwork packages
clone git github.com/docker/libnetwork f72ad20491e8c46d9664da3f32a0eddb301e7c8d
clone git github.com/vishvananda/netns 008d17ae001344769b031375bdb38a86219154c6
clone git github.com/vishvananda/netlink 8eb64238879fed52fd51c5b30ad20b928fb4c36c

# get distribution packages
clone git github.com/docker/distribution b9eeb328080d367dbde850ec6e94f1e4ac2b5efe

clone git github.com/docker/libcontainer v2.1.0
# libcontainer deps (see src/github.com/docker/libcontainer/update-vendor.sh)
clone git github.com/coreos/go-systemd v2
clone git github.com/godbus/dbus v2
clone git github.com/syndtr/gocapability 66ef2aa7a23ba682594e2b6f74cf40c0692b49fb
clone git github.com/golang/protobuf 655cdfa588ea

# List subpackages, exclusing contrib, and the project root
DOCKER_PKG='github.com/docker/docker'
DOCKER_PACKAGES=$(go list github.com/docker/docker/... | grep -v "contrib/" | grep -v "vendor/" | grep -v "^$DOCKER_PKG$")
GODEPJSON_FILES=$(for i in $DOCKER_PACKAGES; do echo $i | sed "s/.*/\/go\/src\/&\/Godeps\/Godeps.json/"; done | tr '\n' ' ')

# Generate Godeps in each subpackage
for i in $DOCKER_PACKAGES; do echo $i && (cd /go/src/$i && GOPATH=/go/:/go/src/$DOCKER_PKG/vendor godep save $i); done

# Merge all Godeps directories in one
for i in $DOCKER_PACKAGES; do cp -R /go/src/$i/Godeps /go/src/$DOCKER_PKG; done

# Merge all Godeps.json files in one
jq --slurp "{
		\"ImportPath\": \"$DOCKER_PKG\",
		\"GoVersion\": \"go1.4.2\",
		\"Deps\": map(.Deps | select(length > 0)) | add | unique_by(.ImportPath)
	}" \
	$GODEPJSON_FILES > Godeps/Godeps.json

# Cleanup
rm -rf vendor
for i in $DOCKER_PACKAGES; do rm -rf /go/src/$i/Godeps; done
