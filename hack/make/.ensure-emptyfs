#!/usr/bin/env bash
set -e

if ! docker image inspect emptyfs > /dev/null; then
	# build a "docker save" tarball for "emptyfs"
	# see https://github.com/docker/docker/pull/5262
	# and also https://github.com/docker/docker/issues/4242
	dir="$DEST/emptyfs"
	uuid=511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158
	mkdir -p "$dir/$uuid"
	(
		echo '{"emptyfs":{"latest":"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158"}}' > "$dir/repositories"
		cd "$dir/$uuid"
		echo '{"id":"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158","comment":"Imported from -","created":"2013-06-13T14:03:50.821769-07:00","container_config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"NetworkDisabled":false,"OnBuild":null},"docker_version":"0.4.0","architecture":"x86_64","Size":0}' > json
		echo '1.0' > VERSION
		tar -cf layer.tar --files-from /dev/null
	)
	(
		[ -n "$TESTDEBUG" ] && set -x
		tar -cC "$dir" . | docker load
	)
	rm -rf "$dir"
fi
