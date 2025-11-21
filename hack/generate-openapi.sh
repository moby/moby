#!/bin/bash
# vim: set noexpandtab:
# -*- indent-tabs-mode: t -*-
set -eu

generate_model() {
	local package="$1"
	shift
	codegen model --spec=api/openapi.yaml \
		--target=api/types --model-package="$package" \
		"$@"
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

generate_model build \
	BuildCacheDiskUsage

generate_model common \
	ErrorResponse \
	IDResponse

generate_model container \
	ChangeType \
	ContainerCreateResponse \
	ContainerTopResponse \
	ContainerUpdateResponse \
	ContainerWaitExitError \
	ContainerWaitResponse \
	ContainersDiskUsage \
	FilesystemChange \
	PortSummary

generate_model image \
	ImageDeleteResponseItem \
	ImagesDiskUsage \
	ImageHistoryItem
#	ImageSummary
# TODO: Restore when go-swagger is updated
# See https://github.com/moby/moby/pull/47526#discussion_r1551800022

generate_model network \
	ConfigReference \
	EndpointResource \
	IPAMStatus \
	Network \
	NetworkConnectRequest \
	NetworkCreateResponse \
	NetworkDisconnectRequest \
	NetworkInspect \
	NetworkStatus \
	NetworkSummary \
	NetworkTaskInfo \
	PeerInfo \
	ServiceInfo \
	SubnetStatus

generate_model plugin \
	Plugin \
	PluginDevice \
	PluginEnv \
	PluginMount \
	PluginConfig \
	PluginEnv \
	PluginArgs \
	PluginInterface \
	LinuxConfig \
	PluginNetwork \
	PluginRootFS \
	PluginInterface \
	PluginUser \
	PluginSettings \

generate_model registry \
	AuthResponse

generate_model storage \
	DriverData \
	RootFSStorage \
	RootFSStorageSnapshot \
	Storage

generate_model swarm \
	ServiceCreateResponse \
	ServiceUpdateResponse

generate_model volume \
	Volume \
	VolumeCreateRequest \
	VolumeListResponse \
	VolumesDiskUsage \
	UsageData

#endregion
