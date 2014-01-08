#!/bin/bash

# Downloads dependencies into vendor/ directory
if [[ ! -d vendor ]]; then
  mkdir vendor
fi
vendor_dir=${PWD}/vendor

rm_pkg_dir () {
  PKG=$1
  REV=$2
  (
    set -e
    cd $vendor_dir
    if [[ -d src/$PKG ]]; then
      echo "src/$PKG already exists. Removing."
      rm -fr src/$PKG
    fi
  )
}

git_clone () {
  PKG=$1
  REV=$2
  (
    set -e
    rm_pkg_dir $PKG $REV
    cd $vendor_dir && git clone http://$PKG src/$PKG
    cd src/$PKG && git checkout -f $REV && rm -fr .git
  )
}

hg_clone () {
  PKG=$1
  REV=$2
  (
    set -e
    rm_pkg_dir $PKG $REV
    cd $vendor_dir && hg clone http://$PKG src/$PKG
    cd src/$PKG && hg checkout -r $REV && rm -fr .hg
  )
}

git_clone github.com/kr/pty 3b1f6487b

git_clone github.com/gorilla/context/ 708054d61e5

git_clone github.com/gorilla/mux/ 9b36453141c

git_clone github.com/syndtr/gocapability 3454319be2

hg_clone code.google.com/p/go.net 84a4013f96e0

hg_clone code.google.com/p/gosqlite 74691fb6f837
