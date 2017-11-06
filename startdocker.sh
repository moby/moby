#hack/make.sh binary
sudo cp bundles/latest/binary-daemon/docker* /usr/bin/
echo "Starting docker................"
sudo dockerd -H tcp://0.0.0.0:2375 -H unix:///var/run/docker.sock -D&

