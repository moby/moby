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

clone git github.com/kr/pty 98c7b80083

clone git github.com/gorilla/context 708054d61e5

clone git github.com/gorilla/mux 9b36453141c

clone git github.com/syndtr/gocapability 3454319be2

clone hg code.google.com/p/go.net 84a4013f96e0

clone hg code.google.com/p/gosqlite 74691fb6f837

# get Go tip's archive/tar, for xattr support
# TODO after Go 1.3 drops, bump our minimum supported version and drop this vendored dep
clone hg code.google.com/p/go a15f344a9efa
mv src/code.google.com/p/go/src/pkg/archive/tar tmp-tar
rm -rf src/code.google.com/p/go
mkdir -p src/code.google.com/p/go/src/pkg/archive
mv tmp-tar src/code.google.com/p/go/src/pkg/archive/tar

clone git github.com/godbus/dbus cb98efbb933d8389ab549a060e880ea3c375d213
clone git github.com/coreos/go-systemd 4c14ed39b8a643ac44b4f95b5a53c00e94261475
