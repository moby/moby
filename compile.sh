./delete-image.sh ubuntu
pkill dockerd
hack/make.sh binary
cp bundles/1.14.0-dev/binary-client/docker* /usr/bin/
cp bundles/1.14.0-dev/binary-daemon/docker* /usr/bin/
docker daemon &
