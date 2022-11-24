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

package cni

import (
	"fmt"
	"os"
	"path"
	"testing"
)

func makeTmpDir(prefix string) (string, error) {
	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	return tmpDir, nil
}

func makeFakeCNIConfig(t *testing.T) (string, string) {
	cniDir, err := makeTmpDir("fakecni")
	if err != nil {
		t.Fatalf("Failed to create plugin config dir: %v", err)
	}

	cniConfDir := path.Join(cniDir, "net.d")
	err = os.MkdirAll(cniConfDir, 0777)
	if err != nil {
		t.Fatalf("Failed to create network config dir: %v", err)
	}

	networkConfig1 := path.Join(cniConfDir, "mocknetwork1.conf")
	f1, err := os.Create(networkConfig1)
	if err != nil {
		t.Fatalf("Failed to create network config %v: %v", f1, err)
	}
	networkConfig2 := path.Join(cniConfDir, "mocknetwork2.conf")
	f2, err := os.Create(networkConfig2)
	if err != nil {
		t.Fatalf("Failed to create network config %v: %v", f2, err)
	}

	cfg1 := fmt.Sprintf(`{ "name": "%s", "type": "%s", "capabilities": {"portMappings": true}  }`, "plugin1", "fakecni")
	_, err = f1.WriteString(cfg1)
	if err != nil {
		t.Fatalf("Failed to write network config file %v: %v", f1, err)
	}
	f1.Close()
	cfg2 := fmt.Sprintf(`{ "name": "%s", "type": "%s", "capabilities": {"portMappings": true}  }`, "plugin2", "fakecni")
	_, err = f2.WriteString(cfg2)
	if err != nil {
		t.Fatalf("Failed to write network config file %v: %v", f2, err)
	}
	f2.Close()
	return cniDir, cniConfDir
}

func tearDownCNIConfig(t *testing.T, confDir string) {
	err := os.RemoveAll(confDir)
	if err != nil {
		t.Fatalf("Failed to cleanup CNI configs: %v", err)
	}
}
