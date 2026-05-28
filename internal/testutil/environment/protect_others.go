//go:build !linux

package environment

import (
	"context"
	"testing"
)

type defaultBridgeInfo struct{}

func ProtectDefaultBridge(context.Context, testing.TB, *Execution) {
	return
}

func restoreDefaultBridge(testing.TB, *defaultBridgeInfo) {}
