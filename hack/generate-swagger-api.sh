#!/bin/sh
set -eu

swagger-gen -o api/types/common-generated.go \
    -f api/swagger.yaml \
    -d ErrorResponse \
    -d IdResponse

swagger-gen -o api/types/images-generated.go \
    -f api/swagger.yaml \
    -d GraphDriverData \
    -d ImageDeleteResponseItem \
    -d ImageSummary

swagger-gen -o api/types/plugins-generated.go \
    -f api/swagger.yaml \
    -d Plugin \
    -d PluginDevice \
    -d PluginMount \
    -d PluginEnv \
    -d PluginInterfaceType

swagger-gen -o api/types/containers-generated.go \
    -f api/swagger.yaml \
    -d Port

swagger-gen -o api/types/services-generated.go \
    -f api/swagger.yaml \
    -d ServiceUpdateResponse

swagger-gen -o api/types/volumes-generated.go \
    -f api/swagger.yaml \
    -d Volume

mkdir -p api/types/registry
swagger-gen --package registry -o api/types/registry/auth-generated.go \
    -f api/swagger.yaml \
    -p SystemAuth

mkdir -p api/types/container
swagger-gen --package container -o api/types/container/responses-generated.go \
    -f api/swagger.yaml \
    -p ContainerChanges \
    -p ContainerCreate \
    -p ContainerTop \
    -p ContainerUpdate \
    -p ContainerWait

mkdir -p api/types/image
swagger-gen --package image -o api/types/image/responses-generated.go \
    -f api/swagger.yaml \
    -p ImageHistory

mkdir -p api/types/volume
swagger-gen --package volume -o api/types/volume/responses-generated.go \
    --definitions-package github.com/docker/docker/api/types \
    -f api/swagger.yaml \
    -p VolumeList
