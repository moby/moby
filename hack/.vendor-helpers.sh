#!/usr/bin/env bash

PROJECT=github.com/docker/docker

# Downloads dependencies into vendor/ directory
mkdir -p vendor

rm -rf .gopath
mkdir -p .gopath/src/github.com/docker
ln -sf ../../../.. .gopath/src/${PROJECT}
export GOPATH="${PWD}/.gopath:${PWD}/vendor"

clone() {
	local vcs="$1"
	local pkg="$2"
	local rev="$3"
	local url="$4"

	: ${url:=https://$pkg}
	local target="vendor/src/$pkg"

	echo -n "$pkg @ $rev: "

	if [ -d "$target" ]; then
		echo -n 'rm old, '
		rm -rf "$target"
	fi

	echo -n 'clone, '
	case "$vcs" in
		git)
			git clone --quiet --no-checkout "$url" "$target"
			( cd "$target" && git checkout --quiet "$rev" && git reset --quiet --hard "$rev" )
			;;
		hg)
			hg clone --quiet --updaterev "$rev" "$url" "$target"
			;;
	esac

	echo -n 'rm VCS, '
	( cd "$target" && rm -rf .{git,hg} )

	echo -n 'rm vendor, '
	( cd "$target" && rm -rf vendor Godeps/_workspace )

	echo done
}

# get an ENV from the Dockerfile with support for multiline values
_dockerfile_env() {
	local e="$1"
	awk '
		$1 == "ENV" && $2 == "'"$e"'" {
			sub(/^ENV +([^ ]+) +/, "");
			inEnv = 1;
		}
		inEnv {
			if (sub(/\\$/, "")) {
				printf "%s", $0;
				next;
			}
			print;
			exit;
		}
	' Dockerfile
}

clean() {
	local packages=(
		"${PROJECT}/docker" # package main
		"${PROJECT}/dockerinit" # package main
		"${PROJECT}/integration-cli" # external tests
	)
	local dockerPlatforms=( linux/amd64 $(_dockerfile_env DOCKER_CROSSPLATFORMS) )
	local dockerBuildTags="$(_dockerfile_env DOCKER_BUILDTAGS)"
	local buildTagCombos=(
		''
		'experimental'
		"$dockerBuildTags"
		"daemon $dockerBuildTags"
		"daemon cgo $dockerBuildTags"
		"experimental $dockerBuildTags"
		"experimental daemon $dockerBuildTags"
		"experimental daemon cgo $dockerBuildTags"
	)

	echo

	echo -n 'collecting import graph, '
	local IFS=$'\n'
	local imports=( $(
		for platform in "${dockerPlatforms[@]}"; do
			export GOOS="${platform%/*}";
			export GOARCH="${platform##*/}";
			for buildTags in "${buildTagCombos[@]}"; do
				pkgs=( $(go list -e -tags "$buildTags" -f '{{join .Deps "\n"}}' "${packages[@]}" | grep -E "^${PROJECT}" | grep -vE "^${PROJECT}/vendor" | sort -u) )
				pkgs+=( ${packages[@]} )
				testImports=( $(go list -e -tags "$buildTags" -f '{{join .TestImports "\n"}}' "${pkgs[@]}" | sort -u) )
				printf '%s\n' "${testImports[@]}"
				go list -e -tags "$buildTags" -f '{{join .Deps "\n"}}' "${packages[@]} ${testImports[@]}"
			done
		done | grep -vE "^${PROJECT}" | sort -u
	) )
	imports=( $(go list -e -f '{{if not .Standard}}{{.ImportPath}}{{end}}' "${imports[@]}") )
	unset IFS

	echo -n 'pruning unused packages, '
	findArgs=(
		# This directory contains only .c and .h files which are necessary
		-path vendor/src/github.com/mattn/go-sqlite3/code
		# This directory is needed for compiling the unit tests
		-o -path vendor/src/github.com/stretchr/objx
	)
	for import in "${imports[@]}"; do
		[ "${#findArgs[@]}" -eq 0 ] || findArgs+=( -or )
		findArgs+=( -path "vendor/src/$import" )
	done
	local IFS=$'\n'
	local prune=( $(find vendor -depth -type d -not '(' "${findArgs[@]}" ')') )
	unset IFS
	for dir in "${prune[@]}"; do
		find "$dir" -maxdepth 1 -not -type d -not -name 'LICENSE*' -not -name 'COPYING*' -exec rm -v -f '{}' +
		rmdir "$dir" 2>/dev/null || true
	done

	echo -n 'pruning unused files, '
	find vendor -type f -name '*_test.go' -exec rm -v '{}' +

	echo done
}

# Fix up hard-coded imports that refer to Godeps paths so they'll work with our vendoring
fix_rewritten_imports () {
       local pkg="$1"
       local remove="${pkg}/Godeps/_workspace/src/"
       local target="vendor/src/$pkg"

       echo "$pkg: fixing rewritten imports"
       find "$target" -name \*.go -exec sed -i -e "s|\"${remove}|\"|g" {} \;
}
