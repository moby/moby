#!/bin/bash
set -e

# hello-world                      latest              ef872312fe1b        3 months ago        910 B
# hello-world                      latest              ef872312fe1bbc5e05aae626791a47ee9b032efa8f3bda39cc0be7b56bfe59b9   3 months ago        910 B

# debian                           latest              f6fab3b798be        10 weeks ago        85.1 MB
# debian                           latest              f6fab3b798be3174f45aa1eb731f8182705555f89c9026d8c1ef230cbf8301dd   10 weeks ago        85.1 MB

if ! command -v curl &> /dev/null; then
	echo >&2 'error: "curl" not found!'
	exit 1
fi
if ! command -v jq &> /dev/null; then
	echo >&2 'error: "jq" not found!'
	exit 1
fi

usage() {
	echo "usage: $0 dir image[:tag][@digest] ..."
	echo "       $0 /tmp/old-hello-world hello-world:latest@sha256:8be990ef2aeb16dbcb9271ddfe2610fa6658d13f6dfb8bc72074cc1ca36966a7"
	[ -z "$1" ] || exit "$1"
}

dir="$1" # dir for building tar in
shift || usage 1 >&2

[ $# -gt 0 -a "$dir" ] || usage 2 >&2
mkdir -p "$dir"

# hacky workarounds for Bash 3 support (no associative arrays)
images=()
rm -f "$dir"/tags-*.tmp
# repositories[busybox]='"latest": "...", "ubuntu-14.04": "..."'

while [ $# -gt 0 ]; do
	imageTag="$1"
	shift
	image="${imageTag%%[:@]*}"
	imageTag="${imageTag#*:}"
	digest="${imageTag##*@}"
	tag="${imageTag%%@*}"

	# add prefix library if passed official image
	if [[ "$image" != *"/"* ]]; then
		image="library/$image"
	fi

	imageFile="${image//\//_}" # "/" can't be in filenames :)

	token="$(curl -sSL "https://auth.docker.io/token?service=registry.docker.io&scope=repository:$image:pull" | jq --raw-output .token)"

	manifestJson="$(curl -sSL -H "Authorization: Bearer $token" "https://registry-1.docker.io/v2/$image/manifests/$digest")"
	if [ "${manifestJson:0:1}" != '{' ]; then
		echo >&2 "error: /v2/$image/manifests/$digest returned something unexpected:"
		echo >&2 "  $manifestJson"
		exit 1
	fi

	layersFs=$(echo "$manifestJson" | jq --raw-output '.fsLayers | .[] | .blobSum')

	IFS=$'\n'
	# bash v4 on Windows CI requires CRLF separator
	if [ "$(go env GOHOSTOS)" = 'windows' ]; then
		major=$(echo ${BASH_VERSION%%[^0.9]} | cut -d. -f1)
		if [ "$major" -ge 4 ]; then
			IFS=$'\r\n'
		fi
	fi
	layers=( ${layersFs} )
	unset IFS

	history=$(echo "$manifestJson" | jq '.history | [.[] | .v1Compatibility]')
	imageId=$(echo "$history" | jq --raw-output .[0] | jq --raw-output .id)

	if [ -s "$dir/tags-$imageFile.tmp" ]; then
		echo -n ', ' >> "$dir/tags-$imageFile.tmp"
	else
		images=( "${images[@]}" "$image" )
	fi
	echo -n '"'"$tag"'": "'"$imageId"'"' >> "$dir/tags-$imageFile.tmp"

	echo "Downloading '${image}:${tag}@${digest}' (${#layers[@]} layers)..."
	for i in "${!layers[@]}"; do
		imageJson=$(echo "$history" | jq --raw-output .[${i}])
		imageId=$(echo "$imageJson" | jq --raw-output .id)
		imageLayer=${layers[$i]}

		mkdir -p "$dir/$imageId"
		echo '1.0' > "$dir/$imageId/VERSION"

		echo "$imageJson" > "$dir/$imageId/json"

		# TODO figure out why "-C -" doesn't work here
		# "curl: (33) HTTP server doesn't seem to support byte ranges. Cannot resume."
		# "HTTP/1.1 416 Requested Range Not Satisfiable"
		if [ -f "$dir/$imageId/layer.tar" ]; then
			# TODO hackpatch for no -C support :'(
			echo "skipping existing ${imageId:0:12}"
			continue
		fi
		token="$(curl -sSL "https://auth.docker.io/token?service=registry.docker.io&scope=repository:$image:pull" | jq --raw-output .token)"
		curl -SL --progress -H "Authorization: Bearer $token" "https://registry-1.docker.io/v2/$image/blobs/$imageLayer" -o "$dir/$imageId/layer.tar" # -C -
	done
	echo
done

echo -n '{' > "$dir/repositories"
firstImage=1
for image in "${images[@]}"; do
	imageFile="${image//\//_}" # "/" can't be in filenames :)
	image="${image#library\/}"

	[ "$firstImage" ] || echo -n ',' >> "$dir/repositories"
	firstImage=
	echo -n $'\n\t' >> "$dir/repositories"
	echo -n '"'"$image"'": { '"$(cat "$dir/tags-$imageFile.tmp")"' }' >> "$dir/repositories"
done
echo -n $'\n}\n' >> "$dir/repositories"

rm -f "$dir"/tags-*.tmp

echo "Download of images into '$dir' complete."
echo "Use something like the following to load the result into a Docker daemon:"
echo "  tar -cC '$dir' . | docker load"
