//go:build !windows

package network

import "github.com/docker/docker/api/types/network"

const defaultNetwork = network.NetworkBridge
