#!/bin/bash
set -e

# usage: ./generate.sh [versions]
#    ie: ./generate.sh
#        to update all Dockerfiles in this directory
#    or: ./generate.sh debian-jessie
#        to only update debian-jessie/Dockerfile
#    or: ./generate.sh debian-newversion
#        to create a new folder and a Dockerfile within it

cd "$(dirname "$(readlink -f "$BASH_SOURCE")")"

versions=( "$@" )
if [ ${#versions[@]} -eq 0 ]; then
	versions=( */ )
fi
versions=( "${versions[@]%/}" )

for version in "${versions[@]}"; do
	mkdir -p "$version"
	distro="${version%-*}"
	suite="${version##*-}"
	from="${distro}:${suite}"
	echo "$version -> FROM $from"
	echo "FROM $from" > "$version/Dockerfile"
	case "$from" in
		debian:wheezy)
			# let's add -backports for wheezy, like our users have to
			echo "RUN echo deb http://http.debian.net/debian $suite-backports main > /etc/apt/sources.list.d/$suite-backports.list" >> "$version/Dockerfile"
			;;
	esac
	echo 'RUN apt-get update && apt-get install -y devscripts && rm -rf /var/lib/apt/lists/*' >> "$version/Dockerfile"
done
