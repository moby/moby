#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
    -t api -m types --skip-validator \
    -n Volume \
    -n Port \
    -n ImageSummary \
    -n Plugin -n PluginDevice -n PluginMount -n PluginEnv -n PluginInterfaceType

swagger generate operation -f api/swagger.yaml \
    -t api -s server -a types -m types \
    -T api/templates --skip-responses --skip-parameters --skip-validator \
    -n VolumesList
