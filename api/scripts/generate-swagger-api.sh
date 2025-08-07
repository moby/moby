#!/bin/sh
set -eu

swagger generate model -f swagger.yaml \
	-t . -m types -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n ErrorResponse \
	-n Plugin \
	-n PluginDevice \
	-n PluginMount \
	-n PluginEnv

swagger generate model -f swagger.yaml \
	-t . -m types/common -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n IDResponse

swagger generate model -f swagger.yaml \
	-t . -m types/storage -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n DriverData

swagger generate model -f swagger.yaml \
	-t . -m types/container -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n ContainerCreateResponse \
	-n ContainerUpdateResponse \
	-n ContainerTopResponse \
	-n ContainerWaitResponse \
	-n ContainerWaitExitError \
	-n ChangeType \
	-n FilesystemChange \
	-n Port

swagger generate model -f swagger.yaml \
	-t . -m types/image -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n ImageDeleteResponseItem
#-n ImageSummary TODO: Restore when go-swagger is updated
# See https://github.com/moby/moby/pull/47526#discussion_r1551800022

swagger generate model -f swagger.yaml \
	-t . -m types/network -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n NetworkCreateResponse

swagger generate model -f swagger.yaml \
	-t . -m types/volume -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n Volume \
	-n VolumeCreateOptions \
	-n VolumeListResponse

swagger generate operation -f swagger.yaml \
	-t . -a types -m types -C swagger-gen.yaml \
	-T templates --allow-template-override \
	--skip-responses --skip-parameters \
	-n Authenticate \
	-n ImageHistory

swagger generate model -f swagger.yaml \
	-t . -m types/swarm -C swagger-gen.yaml \
	-T templates --allow-template-override \
	-n ServiceCreateResponse \
	-n ServiceUpdateResponse
