#!/bin/sh

# try to fetch the latest version from github
wget -O support.sh.new https://raw.githubusercontent.com/docker/libnetwork/master/support/support.sh

if [ "$?" -eq "0" ]; then
  mv support.sh.new support.sh
  chmod +x support.sh
else
  echo "issue fetching the latest support.sh, will use the container version"
fi

echo "run the support script"
./support.sh
