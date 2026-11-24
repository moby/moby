/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package keyring

import (
	"fmt"

	"golang.org/x/sys/unix"
)

type KeySerial int32

func CheckKeyringSupport() error {
	_, err := unix.KeyctlSearch(unix.KEY_SPEC_THREAD_KEYRING, "logon", "dummy", 0)
	if err == unix.ENOSYS {
		return fmt.Errorf("kernel keyring not supported")
	}
	return nil
}

func AddKeyToThreadKeyring(keyType, description string, payload []byte) (KeySerial, error) {
	keyID, err := unix.AddKey(keyType, description, payload, unix.KEY_SPEC_THREAD_KEYRING)
	if err != nil {
		return 0, fmt.Errorf("add_key syscall failed: %w", err)
	}

	return KeySerial(keyID), nil
}

func UnlinkKeyFromThreadKeyring(keyID KeySerial) error {
	_, err := unix.KeyctlInt(unix.KEYCTL_UNLINK, int(keyID), unix.KEY_SPEC_THREAD_KEYRING, 0, 0)
	if err != nil {
		return fmt.Errorf("keyctl unlink failed: %w", err)
	}

	return nil
}
