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

package fs

import (
	"errors"
	"fmt"
	"os"

	winio "github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

const (
	seTakeOwnershipPrivilege = "SeTakeOwnershipPrivilege"
)

func copyFileInfo(fi os.FileInfo, src, name string) error {
	if err := os.Chmod(name, fi.Mode()); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", name, err)
	}

	// Copy file ownership and ACL
	// We need SeRestorePrivilege and SeTakeOwnershipPrivilege in order
	// to restore security info on a file, especially if we're trying to
	// apply security info which includes SIDs not necessarily present on
	// the host.
	privileges := []string{winio.SeRestorePrivilege, seTakeOwnershipPrivilege}
	if err := winio.EnableProcessPrivileges(privileges); err != nil {
		return err
	}
	defer winio.DisableProcessPrivileges(privileges)

	secInfo, err := windows.GetNamedSecurityInfo(
		src, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}

	dacl, _, err := secInfo.DACL()
	if err != nil {
		return err
	}

	sid, _, err := secInfo.Owner()
	if err != nil {
		return err
	}

	if err := windows.SetNamedSecurityInfo(
		name, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
		sid, nil, dacl, nil); err != nil {
		return err
	}
	return nil
}

func copyXAttrs(dst, src string, excludes map[string]struct{}, errorHandler XAttrErrorHandler) error {
	return nil
}

func copyIrregular(dst string, fi os.FileInfo) error {
	return errors.New("irregular copy not supported")
}
