#!/bin/sh
set -eu

swagger generate model -f api/swagger.yaml \
	-t api -m types/plugin -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n Plugin \
	-n PluginDevice \
	-n PluginMount \
	-n PluginEnv

swagger generate model -f api/swagger.yaml \
	-t api -m types/common -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n IDResponse \
	-n ErrorResponse

swagger generate model -f api/swagger.yaml \
	-t api -m types/storage -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n DriverData

swagger generate model -f api/swagger.yaml \
	-t api -m types/container -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n ContainerCreateResponse \
	-n ContainerUpdateResponse \
	-n ContainerTopResponse \
	-n ContainerWaitResponse \
	-n ContainerWaitExitError \
	-n ChangeType \
	-n FilesystemChange \
	-n PortSummary

swagger generate model -f api/swagger.yaml \
	-t api -m types/image -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n ImageDeleteResponseItem
#-n ImageSummary TODO: Restore when go-swagger is updated
# See https://github.com/moby/moby/pull/47526#discussion_r1551800022

swagger generate model -f api/swagger.yaml \
	-t api -m types/network -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n NetworkCreateResponse

swagger generate model -f api/swagger.yaml \
	-t api -m types/volume -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n Volume \
	-n VolumeCreateOptions \
	-n VolumeListResponse

swagger generate operation -f api/swagger.yaml \
	-t api -a types -m types -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	--skip-responses --skip-parameters \
	-n Authenticate \
	-n ImageHistory

swagger generate model -f api/swagger.yaml \
	-t api -m types/swarm -C api/swagger-gen.yaml \
	-T api/templates --allow-template-override \
	-n ServiceCreateResponse \
	-n ServiceUpdateResponse
