#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
	-t api -m types -C api/swagger-gen.yaml \
	-n ErrorResponse \
	-n GraphDriverData \
	-n IdResponse \
	-n Plugin \
	-n PluginDevice \
	-n PluginMount \
	-n PluginEnv \
	-n PluginInterfaceType \
	-n Port

swagger generate model -f api/swagger.yaml \
	-t api -m types/container -C api/swagger-gen.yaml \
	-n ContainerCreateResponse \
	-n ContainerWaitResponse \
	-n ContainerWaitExitError \
	-n ChangeType \
	-n FilesystemChange

swagger generate model -f api/swagger.yaml \
	-t api -m types/image -C api/swagger-gen.yaml \
	-n ImageDeleteResponseItem \
	-n ImageSummary

swagger generate model -f api/swagger.yaml \
	-t api -m types/volume -C api/swagger-gen.yaml \
	-n Volume \
	-n VolumeCreateOptions \
	-n VolumeListResponse

swagger generate operation -f api/swagger.yaml \
	-t api -a types -m types -C api/swagger-gen.yaml \
	-T api/templates --skip-responses --skip-parameters \
	-n Authenticate \
	-n ContainerTop \
	-n ContainerUpdate \
	-n ImageHistory

swagger generate model -f api/swagger.yaml \
	-t api -m types/swarm -C api/swagger-gen.yaml \
	-n ServiceCreateResponse \
	-n ServiceUpdateResponse
