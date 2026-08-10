//go:build linux

package daemon

import (
	"bytes"
	"os"
	"strconv"
)

const (
	rootKeyFile   = "/proc/sys/kernel/keys/root_maxkeys"
	rootBytesFile = "/proc/sys/kernel/keys/root_maxbytes"

	// These match the upstream Linux defaults for key_quota_root_maxkeys
	// and key_quota_root_maxbytes since kernel 3.17.
	//
	// https://github.com/torvalds/linux/commit/738c5d190f6540539a04baf36ce21d46b5da04bd
	// https://github.com/torvalds/linux/blob/738c5d190f6540539a04baf36ce21d46b5da04bd/security/keys/key.c#L30-L33
	rootMaxKeys  = 1000000
	rootMaxBytes = 25 * rootMaxKeys
)

// modifyRootKeyLimit raises the root user's key quota to the upstream
// Linux defaults introduced in kernel 3.17 if the configured limit is
// lower. This avoids exhausting the root key quota on older kernels or
// systems configured with legacy limits.
//
// see https://github.com/moby/moby/issues/22865
func modifyRootKeyLimit() error {
	value, err := readRootKeyLimit(rootKeyFile)
	if err != nil {
		return err
	}
	if value < rootMaxKeys {
		if err := os.WriteFile(rootKeyFile, []byte(strconv.Itoa(rootMaxKeys)), 0); err != nil {
			return err
		}
		if err := os.WriteFile(rootBytesFile, []byte(strconv.Itoa(rootMaxBytes)), 0); err != nil {
			return err
		}
	}
	return nil
}

func readRootKeyLimit(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return strconv.Atoi(string(bytes.TrimSuffix(data, []byte{'\n'})))
}
