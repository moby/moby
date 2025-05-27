package apparmor

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// testAppArmorProfiles fixture "/sys/kernel/security/apparmor/profiles"
// from an Ubuntu 24.10 host.
const testAppArmorProfiles = `wpcom (unconfined)
wike (unconfined)
vpnns (unconfined)
vivaldi-bin (unconfined)
virtiofsd (unconfined)
vdens (unconfined)
uwsgi-core (unconfined)
rsyslogd (enforce)
/usr/lib/snapd/snap-confine (enforce)
/usr/lib/snapd/snap-confine//mount-namespace-capture-helper (enforce)
tcpdump (enforce)
man_groff (enforce)
man_filter (enforce)
/usr/bin/man (enforce)
userbindmount (unconfined)
unprivileged_userns (enforce)
unix-chkpwd (enforce)
ubuntu_pro_esm_cache_systemd_detect_virt (enforce)
ubuntu_pro_esm_cache_systemctl (enforce)
ubuntu_pro_esm_cache (enforce)
ubuntu_pro_esm_cache//ubuntu_distro_info (enforce)
ubuntu_pro_esm_cache//ps (enforce)
ubuntu_pro_esm_cache//dpkg (enforce)
ubuntu_pro_esm_cache//cloud_id (enforce)
ubuntu_pro_esm_cache//apt_methods_gpgv (enforce)
ubuntu_pro_esm_cache//apt_methods (enforce)
ubuntu_pro_apt_news (enforce)
tuxedo-control-center (unconfined)
tup (unconfined)
trinity (unconfined)
transmission-qt (complain)
transmission-gtk (complain)
transmission-daemon (complain)
transmission-cli (complain)
toybox (unconfined)
thunderbird (unconfined)
systemd-coredump (unconfined)
surfshark (unconfined)
stress-ng (unconfined)
steam (unconfined)
slirp4netns (unconfined)
slack (unconfined)
signal-desktop (unconfined)
scide (unconfined)
sbuild-upgrade (unconfined)
sbuild-update (unconfined)
sbuild-unhold (unconfined)
sbuild-shell (unconfined)
sbuild-hold (unconfined)
sbuild-distupgrade (unconfined)
sbuild-destroychroot (unconfined)
sbuild-createchroot (unconfined)
sbuild-clean (unconfined)
sbuild-checkpackages (unconfined)
sbuild-apt (unconfined)
sbuild-adduser (unconfined)
sbuild-abort (unconfined)
sbuild (unconfined)
runc (unconfined)
rssguard (unconfined)
rpm (unconfined)
rootlesskit (unconfined)
qutebrowser (unconfined)
qmapshack (unconfined)
qcam (unconfined)
privacybrowser (unconfined)
polypane (unconfined)
podman (unconfined)
plasmashell (enforce)
plasmashell//QtWebEngineProcess (enforce)
pageedit (unconfined)
opera (unconfined)
opam (unconfined)
obsidian (unconfined)
nvidia_modprobe (enforce)
nvidia_modprobe//kmod (enforce)
notepadqq (unconfined)
nautilus (unconfined)
msedge (unconfined)
mmdebstrap (unconfined)
lxc-usernsexec (unconfined)
lxc-unshare (unconfined)
lxc-stop (unconfined)
lxc-execute (unconfined)
lxc-destroy (unconfined)
lxc-create (unconfined)
lxc-attach (unconfined)
lsb_release (enforce)
loupe (unconfined)
linux-sandbox (unconfined)
libcamerify (unconfined)
lc-compliance (unconfined)
keybase (unconfined)
kchmviewer (unconfined)
ipa_verify (unconfined)
goldendict (unconfined)
github-desktop (unconfined)
geary (unconfined)
foliate (unconfined)
flatpak (unconfined)
firefox (unconfined)
evolution (unconfined)
epiphany (unconfined)
element-desktop (unconfined)
devhelp (unconfined)
crun (unconfined)
vscode (unconfined)
chromium (unconfined)
chrome (unconfined)
ch-run (unconfined)
ch-checkns (unconfined)
cam (unconfined)
busybox (unconfined)
buildah (unconfined)
brave (unconfined)
balena-etcher (unconfined)
Xorg (complain)
QtWebEngineProcess (unconfined)
MongoDB Compass (unconfined)
Discord (unconfined)
1password (unconfined)
`

func TestIsLoaded(t *testing.T) {
	tmpDir := t.TempDir()
	profiles := path.Join(tmpDir, "apparmor_profiles")
	if err := os.WriteFile(profiles, []byte(testAppArmorProfiles), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Run("loaded", func(t *testing.T) {
		found, err := isLoaded("busybox", profiles)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatal("expected profile to be loaded")
		}
	})
	t.Run("not loaded", func(t *testing.T) {
		found, err := isLoaded("no-such-profile", profiles)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			t.Fatal("expected profile to not be loaded")
		}
	})
	t.Run("error", func(t *testing.T) {
		_, err := isLoaded("anything", path.Join(tmpDir, "no_such_file"))
		if err == nil || !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected error to be os.ErrNotExist, got %v", err)
		}
	})
}

func createTestProfiles(b *testing.B, lines int, targetProfile string) string {
	b.Helper()

	var sb strings.Builder
	for i := 0; i < lines-1; i++ {
		sb.WriteString("someprofile (enforcing)\n")
	}
	sb.WriteString(targetProfile + " (enforcing)\n")

	fileName := filepath.Join(b.TempDir(), "apparmor_profiles")
	if err := os.WriteFile(fileName, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	return fileName
}

func BenchmarkIsLoaded(b *testing.B) {
	const target = "myprofile"
	profiles := createTestProfiles(b, 10000, target)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found, err := isLoaded(target, profiles)
		if err != nil || !found {
			b.Fatalf("expected profile to be found, got found=%v, err=%v", found, err)
		}
	}
}
