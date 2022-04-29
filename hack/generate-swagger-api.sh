#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
	-t api -m types --skip-validator -C api/swagger-gen.yaml \
	-n ErrorResponse \
	-n GraphDriverData \
	-n IdResponse \
	-n ImageDeleteResponseItem \
	-n ImageSummary \
	-n Plugin \
	-n PluginDevice \
	-n PluginMount \
	-n PluginEnv \
	-n PluginInterfaceType \
	-n Port \
	-n ServiceUpdateResponse

swagger generate model -f api/swagger.yaml \
	-t api -m types/container --skip-validator -C api/swagger-gen.yaml \
	-n ContainerCreateResponse \
	-n ContainerWaitResponse \
	-n ContainerWaitExitError

swagger generate model -f api/swagger.yaml \
	-t api -m types/volume --skip-validator -C api/swagger-gen.yaml \
	-n Volume \
	-n VolumeCreateOptions \
	-n VolumeListResponse

swagger generate operation -f api/swagger.yaml \
	-t api -a types -m types -C api/swagger-gen.yaml \
	-T api/templates --skip-responses --skip-parameters --skip-validator \
	-n Authenticate \
	-n ContainerChanges \
	-n ContainerTop \
	-n ContainerUpdate \
	-n ImageHistory
