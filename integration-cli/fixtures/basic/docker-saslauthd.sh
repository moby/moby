#!/bin/sh
#
#  At startup, saslauthd changes its working directory, and the relative
#  path for the Kerberos configuration file that we use won't be correct
#  after it does.  Change it to an absolute path.
#
startwd="`pwd`"
KRB5_CONFIG="$startwd"/"${KRB5_CONFIG}"
export KRB5_CONFIG
exec saslauthd "$@"
