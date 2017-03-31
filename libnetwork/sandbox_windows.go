// +build windows

package libnetwork

import "github.com/docker/libnetwork/osl"

func (sb *sandbox) Key() string {
	if sb.config.useDefaultSandBox {
		return osl.GenerateKey("default")
	}
	return osl.GenerateKey(sb.containerID)
}
