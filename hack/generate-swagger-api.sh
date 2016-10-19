#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
    -t api -m types --skip-validator -C api/swagger-gen.yaml \
    -n Volume \
    -n Port \
    -n ImageSummary \
    -n Plugin -n PluginDevice -n PluginMount -n PluginEnv -n PluginInterfaceType \
    -n ErrorResponse \
    -n IdResponse

swagger generate operation -f api/swagger.yaml \
    -t api -a types -m types -C api/swagger-gen.yaml \
    -T api/templates --skip-responses --skip-parameters --skip-validator \
    -n VolumesList \
    -n VolumesCreate \
    -n ContainerCreate \
    -n ContainerUpdate
