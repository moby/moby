#!/bin/sh

# This script sets common build options
SCRIPT_DIR=`dirname "$0"`
source $SCRIPT_DIR/version.sh

LDFLAGS="-X main.GITCOMMIT $GITCOMMIT -X main.VERSION $VERSION -d -w"

