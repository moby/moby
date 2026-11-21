package windows

import "testing"

func TestScratchVolumeDevice(t *testing.T) {
	for _, tc := range []struct {
		mountPath, device string
		ok                bool
	}{
		{`\\?\Volume{5cc80af7-5b47-49f9-a45c-f6c28bec13bc}\`, `\\?\Volume{5cc80af7-5b47-49f9-a45c-f6c28bec13bc}`, true},
		{`\\?\Volume{5cc80af7-5b47-49f9-a45c-f6c28bec13bc}`, `\\?\Volume{5cc80af7-5b47-49f9-a45c-f6c28bec13bc}`, true},
		{`\\?\Volume{5cc80af7`, "", false},
		{`C:\ProgramData\docker\windowsfilter\0123abcd`, "", false},
		{"", "", false},
	} {
		device, ok := scratchVolumeDevice(tc.mountPath)
		if device != tc.device || ok != tc.ok {
			t.Errorf("scratchVolumeDevice(%q) = %q, %v; want %q, %v", tc.mountPath, device, ok, tc.device, tc.ok)
		}
	}
}
