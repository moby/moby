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

package utils

import (
	"context"
	"crypto/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"
)

func RequireTool(t *testing.T, name string) string {
	t.Helper()
	p, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("%s not found in PATH", name)
	}
	return p
}

func RequireRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatalf("requires root")
	}
}

func RunCmd(t *testing.T, name string, args ...string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
	return string(out), string(out)
}

func RunGoCLI(t *testing.T, args ...string) (string, string) {
	t.Helper()
	bin := RequireTool(t, "go-dmverity")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", bin, args, err, string(out))
	}
	return string(out), string(out)
}

func MakeTempFile(t *testing.T, size int64) string {
	t.Helper()
	f, err := os.CreateTemp("", "vgo-data-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if size > 0 {
		if err := f.Truncate(size); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4096)
		if _, err := rand.Read(buf); err == nil {
			_, _ = f.WriteAt(buf, 0)
		}
	}
	return f.Name()
}

func FirstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return s[:i]
		}
	}
	return s
}

func ExtractRootHex(t *testing.T, out string) string {
	t.Helper()
	re := regexp.MustCompile(`(?i)Root hash:\s*([0-9a-f]+)`)
	m := re.FindStringSubmatch(out)
	if len(m) < 2 {
		t.Fatalf("failed to parse root hash from output: %s", out)
	}
	return m[1]
}

func CreateFormattedFiles(t *testing.T) (dataPath, hashPath, rootHex string) {
	t.Helper()
	dataPath = MakeTempFile(t, 4096*16)
	f, err := os.CreateTemp("", "vgo-hash-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	hashPath = f.Name()
	out, _ := RunGoCLI(t, "format", "--hash", "sha256", "--data-block-size", "4096", "--hash-block-size", "4096", "--salt", "-", "--no-superblock", "--hash-offset", "0", dataPath, hashPath)
	rootHex = ExtractRootHex(t, out)
	return
}

func MaskVerityDevices(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 7 {
			vIdx := -1
			for idx, tok := range fields {
				if tok == "verity" {
					vIdx = idx
					break
				}
			}
			if vIdx >= 0 && vIdx+3 < len(fields) {
				fields[vIdx+2] = "<dev>"
				fields[vIdx+3] = "<dev>"
				lines[i] = strings.Join(fields, " ")
				continue
			}
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

type VerityTestParams struct {
	Hash          string
	DataBlockSize string
	HashBlockSize string
	Salt          string
	NoSuperblock  bool
	HashOffset    string
}

func DefaultVerityTestParams() VerityTestParams {
	return VerityTestParams{
		Hash:          "sha256",
		DataBlockSize: "4096",
		HashBlockSize: "4096",
		Salt:          "-",
		NoSuperblock:  true,
		HashOffset:    "0",
	}
}

func (p VerityTestParams) BuildOpenArgs(dataLoop, name, hashLoop, rootHex string) []string {
	args := []string{"open"}
	if p.Hash != "" {
		args = append(args, "--hash", p.Hash)
	}
	if p.DataBlockSize != "" {
		args = append(args, "--data-block-size", p.DataBlockSize)
	}
	if p.HashBlockSize != "" {
		args = append(args, "--hash-block-size", p.HashBlockSize)
	}
	if p.Salt != "" {
		args = append(args, "--salt", p.Salt)
	}
	if p.NoSuperblock {
		args = append(args, "--no-superblock")
		if p.HashOffset != "" {
			args = append(args, "--hash-offset", p.HashOffset)
		}
	}
	args = append(args, dataLoop, name, hashLoop, rootHex)
	return args
}

func (p VerityTestParams) BuildVerifyArgs(data, hash, rootHex string) []string {
	args := []string{"verify"}
	if p.Hash != "" {
		args = append(args, "--hash", p.Hash)
	}
	if p.DataBlockSize != "" {
		args = append(args, "--data-block-size", p.DataBlockSize)
	}
	if p.HashBlockSize != "" {
		args = append(args, "--hash-block-size", p.HashBlockSize)
	}
	if p.Salt != "" {
		args = append(args, "--salt", p.Salt)
	}
	if p.NoSuperblock {
		args = append(args, "--no-superblock")
		if p.HashOffset != "" {
			args = append(args, "--hash-offset", p.HashOffset)
		}
	}
	args = append(args, data, hash, rootHex)
	return args
}

func (p VerityTestParams) BuildCVeritysetupOpenArgs(dataLoop, name, hashLoop, rootHex string) []string {
	args := []string{"--debug", "--verbose", "open", dataLoop, name, hashLoop, rootHex}
	if p.Hash != "" {
		args = append(args, "--hash", p.Hash)
	}
	if p.DataBlockSize != "" {
		args = append(args, "--data-block-size", p.DataBlockSize)
	}
	if p.HashBlockSize != "" {
		args = append(args, "--hash-block-size", p.HashBlockSize)
	}
	if p.Salt != "" {
		args = append(args, "--salt", p.Salt)
	}
	if p.NoSuperblock {
		args = append(args, "--no-superblock")
	}
	return args
}

func (p VerityTestParams) BuildCVeritysetupVerifyArgs(data, hash, rootHex string) []string {
	args := []string{"--debug", "--verbose", "verify", data, hash, rootHex}
	if p.Hash != "" {
		args = append(args, "--hash", p.Hash)
	}
	if p.DataBlockSize != "" {
		args = append(args, "--data-block-size", p.DataBlockSize)
	}
	if p.HashBlockSize != "" {
		args = append(args, "--hash-block-size", p.HashBlockSize)
	}
	if p.Salt != "" {
		args = append(args, "--salt", p.Salt)
	}
	if p.NoSuperblock {
		args = append(args, "--no-superblock")
	}
	return args
}

type DMDeviceCleanup struct {
	t       *testing.T
	devices []string
}

func NewDMDeviceCleanup(t *testing.T) *DMDeviceCleanup {
	t.Helper()
	return &DMDeviceCleanup{t: t, devices: make([]string, 0)}
}

func (c *DMDeviceCleanup) Add(device string) string {
	c.devices = append(c.devices, device)
	return device
}

func (c *DMDeviceCleanup) Cleanup() {
	for _, device := range c.devices {
		if err := exec.Command("dmsetup", "remove", device).Run(); err != nil {
			c.t.Logf("dmsetup remove %s: %v", device, err)
		}
	}
}

func OpenVerityDevice(t *testing.T, params VerityTestParams, dataLoop, name, hashLoop, rootHex string) {
	t.Helper()
	args := params.BuildOpenArgs(dataLoop, name, hashLoop, rootHex)
	RunGoCLI(t, args...)
}

func VerifyDeviceRemoved(t *testing.T, name string) {
	t.Helper()
	if out, err := exec.Command("dmsetup", "info", name).CombinedOutput(); err == nil {
		t.Fatalf("expected %s to be removed, but dmsetup info succeeded: %s", name, string(out))
	}
}
