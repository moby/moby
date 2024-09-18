//go:build linux || freebsd

package operatingsystem // import "github.com/docker/docker/pkg/parsers/operatingsystem"

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

type EtcReleaseParsingTest struct {
	name        string
	content     string
	expected    string
	expectedErr string
}

func TestGetOperatingSystem(t *testing.T) {
	tests := []EtcReleaseParsingTest{
		{
			content: `NAME="Ubuntu"
PRETTY_NAME_AGAIN="Ubuntu 14.04.LTS"
VERSION="14.04, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`,
			expected: "Linux",
		},
		{
			content: `NAME="Ubuntu"
VERSION="14.04, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`,
			expected: "Linux",
		},
		{
			content: `NAME=Gentoo
ID=gentoo
PRETTY_NAME="Gentoo/Linux"
ANSI_COLOR="1;32"
HOME_URL="http://www.gentoo.org/"
SUPPORT_URL="http://www.gentoo.org/main/en/support.xml"
BUG_REPORT_URL="https://bugs.gentoo.org/"
`,
			expected: "Gentoo/Linux",
		},
		{
			content: `NAME="Ubuntu"
VERSION="14.04, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 14.04 LTS"
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`,
			expected: "Ubuntu 14.04 LTS",
		},
		{
			content: `NAME="Ubuntu"
VERSION="14.04, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME='Ubuntu 14.04 LTS'`,
			expected: "Ubuntu 14.04 LTS",
		},
		{
			content: `PRETTY_NAME=Source
NAME="Source Mage"`,
			expected: "Source",
		},
		{
			content: `PRETTY_NAME=Source
PRETTY_NAME="Source Mage"`,
			expected: "Source Mage",
		},
	}

	runEtcReleaseParsingTests(t, tests, GetOperatingSystem)
}

func TestGetOperatingSystemVersion(t *testing.T) {
	tests := []EtcReleaseParsingTest{
		{
			name: "ubuntu 14.04",
			content: `NAME="Ubuntu"
PRETTY_NAME="Ubuntu 14.04.LTS"
VERSION="14.04, Trusty Tahr"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="14.04"
HOME_URL="http://www.ubuntu.com/"
SUPPORT_URL="http://help.ubuntu.com/"
BUG_REPORT_URL="http://bugs.launchpad.net/ubuntu/"`,
			expected: "14.04",
		},
		{
			name: "gentoo",
			content: `NAME=Gentoo
ID=gentoo
PRETTY_NAME="Gentoo/Linux"
ANSI_COLOR="1;32"
HOME_URL="http://www.gentoo.org/"
SUPPORT_URL="http://www.gentoo.org/main/en/support.xml"
BUG_REPORT_URL="https://bugs.gentoo.org/"
`,
		},
		{
			name: "dual version id",
			content: `VERSION_ID="14.04"
VERSION_ID=18.04`,
			expected: "18.04",
		},
	}

	runEtcReleaseParsingTests(t, tests, GetOperatingSystemVersion)
}

func runEtcReleaseParsingTests(t *testing.T, tests []EtcReleaseParsingTest, parsingFunc func() (string, error)) {
	backup := etcOsRelease

	dir := os.TempDir()
	etcOsRelease = filepath.Join(dir, "etcOsRelease")

	defer func() {
		os.Remove(etcOsRelease)
		etcOsRelease = backup
	}()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(etcOsRelease, []byte(test.content), 0o600); err != nil {
				t.Fatalf("failed to write to %s: %v", etcOsRelease, err)
			}
			s, err := parsingFunc()
			if test.expectedErr == "" {
				assert.NilError(t, err)
			} else {
				assert.Error(t, err, test.expectedErr)
			}
			assert.Equal(t, s, test.expected)
		})
	}
}

func TestIsContainerized(t *testing.T) {
	var (
		backup                                = proc1Cgroup
		nonContainerizedProc1Cgroupsystemd226 = []byte(`9:memory:/init.scope
8:net_cls,net_prio:/
7:cpuset:/
6:freezer:/
5:devices:/init.scope
4:blkio:/init.scope
3:cpu,cpuacct:/init.scope
2:perf_event:/
1:name=systemd:/init.scope
`)
		nonContainerizedProc1Cgroup = []byte(`14:name=systemd:/
13:hugetlb:/
12:net_prio:/
11:perf_event:/
10:bfqio:/
9:blkio:/
8:net_cls:/
7:freezer:/
6:devices:/
5:memory:/
4:cpuacct:/
3:cpu:/
2:cpuset:/
`)
		containerizedProc1Cgroup = []byte(`9:perf_event:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
8:blkio:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
7:net_cls:/
6:freezer:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
5:devices:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
4:memory:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
3:cpuacct:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
2:cpu:/docker/3cef1b53c50b0fa357d994f8a1a8cd783c76bbf4f5dd08b226e38a8bd331338d
1:cpuset:/`)
		nonContainerizedProc1CgroupNotSystemd = []byte(`9:memory:/not/init.scope
	1:name=not_systemd:/not.init.scope
`)
	)

	dir := os.TempDir()
	proc1Cgroup = filepath.Join(dir, "proc1Cgroup")

	defer func() {
		os.Remove(proc1Cgroup)
		proc1Cgroup = backup
	}()

	if err := os.WriteFile(proc1Cgroup, nonContainerizedProc1Cgroup, 0o600); err != nil {
		t.Fatalf("failed to write to %s: %v", proc1Cgroup, err)
	}
	inContainer, err := IsContainerized()
	if err != nil {
		t.Fatal(err)
	}
	if inContainer {
		t.Fatal("Wrongly assuming containerized")
	}

	if err := os.WriteFile(proc1Cgroup, nonContainerizedProc1Cgroupsystemd226, 0o600); err != nil {
		t.Fatalf("failed to write to %s: %v", proc1Cgroup, err)
	}
	inContainer, err = IsContainerized()
	if err != nil {
		t.Fatal(err)
	}
	if inContainer {
		t.Fatal("Wrongly assuming containerized for systemd /init.scope cgroup layout")
	}

	if err := os.WriteFile(proc1Cgroup, nonContainerizedProc1CgroupNotSystemd, 0o600); err != nil {
		t.Fatalf("failed to write to %s: %v", proc1Cgroup, err)
	}
	inContainer, err = IsContainerized()
	if err != nil {
		t.Fatal(err)
	}
	if !inContainer {
		t.Fatal("Wrongly assuming non-containerized")
	}

	if err := os.WriteFile(proc1Cgroup, containerizedProc1Cgroup, 0o600); err != nil {
		t.Fatalf("failed to write to %s: %v", proc1Cgroup, err)
	}
	inContainer, err = IsContainerized()
	if err != nil {
		t.Fatal(err)
	}
	if !inContainer {
		t.Fatal("Wrongly assuming non-containerized")
	}
}

func TestOsReleaseFallback(t *testing.T) {
	backup := etcOsRelease
	altBackup := altOsRelease
	dir := os.TempDir()
	etcOsRelease = filepath.Join(dir, "etcOsRelease")
	altOsRelease = filepath.Join(dir, "altOsRelease")

	defer func() {
		os.Remove(dir)
		etcOsRelease = backup
		altOsRelease = altBackup
	}()
	content := `NAME=Gentoo
ID=gentoo
PRETTY_NAME="Gentoo/Linux"
ANSI_COLOR="1;32"
HOME_URL="http://www.gentoo.org/"
SUPPORT_URL="http://www.gentoo.org/main/en/support.xml"
BUG_REPORT_URL="https://bugs.gentoo.org/"
`
	if err := os.WriteFile(altOsRelease, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write to %s: %v", etcOsRelease, err)
	}
	s, err := GetOperatingSystem()
	if err != nil || s != "Gentoo/Linux" {
		t.Fatalf("Expected %q, got %q (err: %v)", "Gentoo/Linux", s, err)
	}
}
