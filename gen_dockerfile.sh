#!/bin/bash -e

VER=$(awk '{ print $1 }' /etc/redhat-release 2>/dev/null) || VER=

case $VER in
    "Fedora")
       		patch -p1 -o Dockerfile$VER < Fedora.patch  > /dev/null
	    ;;
    "Centos")
       		patch -p1 -o Dockerfile$VER < Centos.patch > /dev/null
	    ;;
    "Red")
	VER=RHEL
       	patch -p1 -o Dockerfile$VER < rhel.patch > /dev/null
	;;
esac
echo Dockerfile$VER




