#!/bin/bash

while true; do
	if ! ./hack/make.sh binary; then
		# If the build fails, sleep for 5 seconds and continue
		sleep 5
		continue
	fi
	KEEPBUNDLE=1 ./hack/make.sh install-binary || continue

	dockerd --debug || true
	sleep 0.1
done
