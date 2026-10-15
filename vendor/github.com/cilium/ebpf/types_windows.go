package ebpf

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"

	"github.com/cilium/ebpf/internal/efw"
	"github.com/cilium/ebpf/internal/platform"
)

// WindowsProgramTypeForGUID resolves a GUID to a ProgramType.
//
// The GUID must be in the form of "{XXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}".
//
// Returns an error wrapping [os.ErrNotExist] if the GUID is not recignized.
func WindowsProgramTypeForGUID(guid string) (ProgramType, error) {
	progTypeGUID, err := windows.GUIDFromString(guid)
	if err != nil {
		return 0, fmt.Errorf("parse GUID: %w", err)
	}

	rawProgramType, err := efw.EbpfGetBpfProgramType(progTypeGUID)
	if err != nil {
		return 0, fmt.Errorf("get program type: %w", err)
	}

	if rawProgramType == 0 {
		return 0, fmt.Errorf("program type not found for GUID %v: %w", guid, os.ErrNotExist)
	}

	return ProgramTypeForPlatform(platform.Windows, rawProgramType)
}

// WindowsAttachTypeForGUID resolves a GUID to an AttachType.
//
// The GUID must be in the form of "{XXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}".
//
// Returns an error wrapping [os.ErrNotExist] if the GUID is not recignized.
func WindowsAttachTypeForGUID(guid string) (AttachType, error) {
	attachTypeGUID, err := windows.GUIDFromString(guid)
	if err != nil {
		return 0, fmt.Errorf("parse GUID: %w", err)
	}

	rawAttachType, err := efw.EbpfGetBpfAttachType(attachTypeGUID)
	if err != nil {
		return 0, fmt.Errorf("get attach type: %w", err)
	}

	if rawAttachType == 0 {
		return 0, fmt.Errorf("attach type not found for GUID %v: %w", attachTypeGUID, os.ErrNotExist)
	}

	return AttachTypeForPlatform(platform.Windows, rawAttachType)
}
