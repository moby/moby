#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
    -t api -m types --skip-validator \
    -n Volume \
    -n Port \
    -n ImageSummary \
    -n Plugin -n PluginDevice -n PluginMount -n PluginEnv -n PluginInterfaceType
