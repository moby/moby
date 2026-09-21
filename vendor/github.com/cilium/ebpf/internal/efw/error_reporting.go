//go:build windows

package efw

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

func init() {
	if !testing.Testing() {
		return
	}

	if isDebuggerPresent() {
		return
	}

	if err := configureCRTErrorReporting(); err != nil {
		fmt.Fprintln(os.Stderr, "WARNING: Could not configure CRT error reporting, tests may hang:", err)
	}
}

var errErrorReportingAlreadyConfigured = errors.New("error reporting already configured")

// Configure built-in error reporting of the C runtime library.
//
// The C runtime emits assertion failures into a graphical message box by default.
// This causes a hang in CI environments. This function configures the CRT to
// log to stderr instead.
func configureCRTErrorReporting() error {
	const ucrtDebug = "ucrtbased.dll"

	// Constants from crtdbg.h
	//
	// See https://doxygen.reactos.org/da/d40/crt_2crtdbg_8h_source.html
	const (
		_CRT_ERROR          = 1
		_CRT_ASSERT         = 2
		_CRTDBG_MODE_FILE   = 0x1
		_CRTDBG_MODE_WNDW   = 0x4
		_CRTDBG_HFILE_ERROR = -2
		_CRTDBG_FILE_STDERR = -4
	)

	// Load the efW API to trigger loading the CRT. This may fail, in which case
	// we can't figure out which CRT is being used.
	// In that case we rely on the error bubbling up via some other path.
	_ = module.Load()

	ucrtHandle, err := syscall.UTF16PtrFromString(ucrtDebug)
	if err != nil {
		return err
	}

	var handle windows.Handle
	err = windows.GetModuleHandleEx(0, ucrtHandle, &handle)
	if errors.Is(err, windows.ERROR_MOD_NOT_FOUND) {
		// Loading the ebpf api did not pull in the debug UCRT, so there is
		// nothing to configure.
		return nil
	} else if err != nil {
		return err
	}
	defer windows.FreeLibrary(handle)

	setReportModeAddr, err := windows.GetProcAddress(handle, "_CrtSetReportMode")
	if err != nil {
		return err
	}

	setReportMode := func(reportType int, reportMode int) (int, error) {
		// See https://learn.microsoft.com/en-us/cpp/c-runtime-library/reference/crtsetreportmode?view=msvc-170
		r1, _, err := syscall.SyscallN(setReportModeAddr, uintptr(reportType), uintptr(reportMode))
		if int(r1) == -1 {
			return 0, fmt.Errorf("set report mode for type %d: %w", reportType, err)
		}
		return int(r1), nil
	}

	setReportFileAddr, err := windows.GetProcAddress(handle, "_CrtSetReportFile")
	if err != nil {
		return err
	}

	setReportFile := func(reportType int, reportFile int) (int, error) {
		// See https://learn.microsoft.com/en-us/cpp/c-runtime-library/reference/crtsetreportfile?view=msvc-170
		r1, _, err := syscall.SyscallN(setReportFileAddr, uintptr(reportType), uintptr(reportFile))
		if int(r1) == _CRTDBG_HFILE_ERROR {
			return 0, fmt.Errorf("set report file for type %d: %w", reportType, err)
		}
		return int(r1), nil
	}

	reportToFile := func(reportType, defaultMode int) error {
		oldMode, err := setReportMode(reportType, _CRTDBG_MODE_FILE)
		if err != nil {
			return err
		}

		if oldMode != defaultMode {
			// Attempt to restore old mode if it was different from the expected default.
			_, _ = setReportMode(reportType, oldMode)
			return errErrorReportingAlreadyConfigured
		}

		oldFile, err := setReportFile(reportType, _CRTDBG_FILE_STDERR)
		if err != nil {
			return err
		}

		if oldFile != -1 {
			// Attempt to restore old file if it was different from the expected default.
			_, _ = setReportFile(reportType, oldFile)
			return errErrorReportingAlreadyConfigured
		}

		return nil
	}

	// See https://learn.microsoft.com/en-us/cpp/c-runtime-library/reference/crtsetreportmode?view=msvc-170#remarks
	// for defaults.
	if err := reportToFile(_CRT_ASSERT, _CRTDBG_MODE_WNDW); err != nil {
		return err
	}

	if err := reportToFile(_CRT_ERROR, _CRTDBG_MODE_WNDW); err != nil {
		return err
	}

	return nil
}

// isDebuggerPresent returns true if the current process is being debugged.
//
// See https://learn.microsoft.com/en-us/windows/win32/api/debugapi/nf-debugapi-isdebuggerpresent
func isDebuggerPresent() bool {
	kernel32Handle, err := windows.LoadLibrary("kernel32.dll")
	if err != nil {
		return false
	}

	isDebuggerPresentAddr, err := windows.GetProcAddress(kernel32Handle, "IsDebuggerPresent")
	if err != nil {
		return false
	}

	r1, _, _ := syscall.SyscallN(isDebuggerPresentAddr)
	return r1 != 0
}
