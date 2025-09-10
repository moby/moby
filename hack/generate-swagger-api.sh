#!/bin/bash
# vim: set noexpandtab:
# -*- indent-tabs-mode: t -*-
set -eu

generate_model() {
	local package="$1"
	shift
	mapfile
	swagger generate model --spec=api/swagger.yaml \
		--target=api --model-package="$package" \
		--config-file=api/swagger-gen.yaml \
		--template-dir=api/templates --allow-template-override \
		"$@" \
		$(printf -- '--name=%s ' "${MAPFILE[@]}")
}

generate_operation() {
	mapfile
	swagger generate operation --spec=api/swagger.yaml \
		--target=api --api-package=types --model-package=types \
		--config-file=api/swagger-gen.yaml \
		--template-dir=api/templates --allow-template-override \
		"$@" \
		$(printf -- '--name=%s ' "${MAPFILE[@]}")
}

# /==================================================================\
# |                                                                  |
# |  ATTENTION:                                                      |
# |                                                                  |
# |       Sort model package stanzas and model/operation names       |
# |                    *** ALPHABETICALLY ***                        |
# |          to reduce the likelihood of merge conflicts.            |
# |                                                                  |
# \==================================================================/

#region -------- Models --------

generate_model types/common <<- 'EOT'
	ErrorResponse
	IDResponse
EOT

generate_model types/container <<- 'EOT'
	ChangeType
	ContainerCreateResponse
	ContainerTopResponse
	ContainerUpdateResponse
	ContainerWaitExitError
	ContainerWaitResponse
	FilesystemChange
	PortSummary
EOT

generate_model types/image <<- 'EOT'
	ImageDeleteResponseItem
EOT
#	ImageSummary
# TODO: Restore when go-swagger is updated
# See https://github.com/moby/moby/pull/47526#discussion_r1551800022

generate_model types/network --keep-spec-order --additional-initialism=IPAM <<- 'EOT'
	ConfigReference
	EndpointResource
	IPAMStatus
	Network
	NetworkCreateResponse
	NetworkInspect
	NetworkStatus
	NetworkSummary
	NetworkTaskInfo
	PeerInfo
	ServiceInfo
	SubnetStatus
EOT

generate_model types/plugin <<- 'EOT'
	Plugin
	PluginDevice
	PluginEnv
	PluginMount
EOT

generate_model types/storage <<- 'EOT'
	DriverData
	RootFSStorage
	RootFSStorageSnapshot
	Storage
EOT

generate_model types/swarm <<- 'EOT'
	ServiceCreateResponse
	ServiceUpdateResponse
EOT

generate_model types/volume <<- 'EOT'
	Volume
	VolumeCreateOptions
	VolumeListResponse
EOT

#endregion

#region -------- Operations --------

generate_operation --skip-responses --skip-parameters <<- 'EOT'
	Authenticate
	ImageHistory
EOT

#endregion
