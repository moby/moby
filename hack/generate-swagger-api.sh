#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
	-t api -m types --skip-validator -C api/swagger-gen.yaml \
	-n ErrorResponse \
	-n IdResponse \
	-n Plugin \
	-n PluginDevice \
	-n PluginMount \
	-n PluginEnv \
	-n PluginInterfaceType

swagger generate model -f api/swagger.yaml \
	-t api -m types/storage --skip-validator -C api/swagger-gen.yaml \
	-n DriverData

swagger generate model -f api/swagger.yaml \
	-t api -m types/container --skip-validator -C api/swagger-gen.yaml \
	-n ContainerCreateResponse \
	-n ContainerWaitResponse \
	-n ContainerWaitExitError \
	-n ChangeType \
	-n FilesystemChange \
	-n Port

swagger generate model -f api/swagger.yaml \
	-t api -m types/image --skip-validator -C api/swagger-gen.yaml \
	-n ImageDeleteResponseItem \
	-n ImageSummary

swagger generate model -f api/swagger.yaml \
	-t api -m types/network --skip-validator -C api/swagger-gen.yaml \
	-n NetworkCreateResponse

swagger generate model -f api/swagger.yaml \
	-t api -m types/volume --skip-validator -C api/swagger-gen.yaml \
	-n Volume \
	-n VolumeCreateOptions \
	-n VolumeListResponse

swagger generate operation -f api/swagger.yaml \
	-t api -a types -m types -C api/swagger-gen.yaml \
	-T api/templates --skip-responses --skip-parameters --skip-validator \
	-n Authenticate \
	-n ContainerTop \
	-n ContainerUpdate \
	-n ImageHistory

swagger generate model -f api/swagger.yaml \
	-t api -m types/swarm --skip-validator -C api/swagger-gen.yaml \
	-n ServiceCreateResponse \
	-n ServiceUpdateResponse
