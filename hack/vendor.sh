#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."

# Downloads dependencies into vendor/ directory
mkdir -p vendor
cd vendor

clone() {
	vcs=$1
	pkg=$2
	rev=$3

	pkg_url=https://$pkg
	target_dir=src/$pkg

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

	echo -n 'rm VCS, '
	( cd $target_dir && rm -rf .{git,hg} )

	echo done
}

clone git github.com/kr/pty 05017fcccf

clone git github.com/gorilla/context 14f550f51a

clone git github.com/gorilla/mux e444e69cbd

clone git github.com/tchap/go-patricia v1.0.1

clone hg code.google.com/p/go.net 84a4013f96e0

clone hg code.google.com/p/gosqlite 74691fb6f837

clone git github.com/docker/libtrust 230dfd18c232

clone git github.com/Sirupsen/logrus v0.7.2

clone git github.com/go-fsnotify/fsnotify v1.0.4

clone git github.com/go-check/check 64131543e7896d5bcc6bd5a76287eb75ea96c673

# get distribution packages
clone git github.com/docker/distribution d957768537c5af40e4f4cd96871f7b2bde9e2923
mv src/github.com/docker/distribution/digest tmp-digest
mv src/github.com/docker/distribution/registry/api tmp-api
rm -rf src/github.com/docker/distribution
mkdir -p src/github.com/docker/distribution
mv tmp-digest src/github.com/docker/distribution/digest
mkdir -p src/github.com/docker/distribution/registry
mv tmp-api src/github.com/docker/distribution/registry/api

clone git github.com/docker/libcontainer bd8ec36106086f72b66e1be85a81202b93503e44
# see src/github.com/docker/libcontainer/update-vendor.sh which is the "source of truth" for libcontainer deps (just like this file)
rm -rf src/github.com/docker/libcontainer/vendor
eval "$(grep '^clone ' src/github.com/docker/libcontainer/update-vendor.sh | grep -v 'github.com/codegangsta/cli' | grep -v 'github.com/Sirupsen/logrus')"
# we exclude "github.com/codegangsta/cli" here because it's only needed for "nsinit", which Docker doesn't include
