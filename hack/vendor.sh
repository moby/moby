#!/bin/bash

# Downloads dependencies into vendor/ directory
if [[ ! -d vendor ]]; then
  mkdir vendor
fi
vendor_dir=${PWD}/vendor

git_clone () {
  PKG=$1
  REV=$2
  (
    set -e
    cd $vendor_dir
    if [[ -d src/$PKG ]]; then
      echo "src/$PKG already exists. Removing."
      rm -fr src/$PKG
    fi
    cd $vendor_dir && git clone http://$PKG src/$PKG
    cd src/$PKG && git checkout -f $REV && rm -fr .git
  )
}

git_clone github.com/kr/pty 3b1f6487b

git_clone github.com/gorilla/context/ 708054d61e5

git_clone github.com/gorilla/mux/ 9b36453141c

git_clone github.com/dotcloud/tar/ e5ea6bb21a

# Docker requires code.google.com/p/go.net/websocket
PKG=code.google.com/p/go.net REV=84a4013f96e0
(
  set -e
  cd $vendor_dir
  if [[ ! -d src/$PKG ]]; then
    hg clone https://$PKG src/$PKG
  fi
  cd src/$PKG && hg checkout -r $REV
)
