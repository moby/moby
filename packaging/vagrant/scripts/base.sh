#!/bin/sh

set -e

date > /etc/vagrant_box_build_time

# Prepare the system's package-manager and make it
# minimal.

# Compress apt indexes
cat <<EOF > /etc/apt/apt.conf.d/02compress-indexes
Acquire::GzipIndexes "true";
Acquire::CompressionTypes::Order:: "gz";
EOF
apt-get update

# Install base utils
apt-get install -qy curl
