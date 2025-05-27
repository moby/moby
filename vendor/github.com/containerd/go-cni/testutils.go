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

func makeFakeCNIConfig(t *testing.T) (string, string) {
	cniDir := t.TempDir()

	cniConfDir := path.Join(cniDir, "net.d")
	err := os.MkdirAll(cniConfDir, 0777)
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

func buildFakeConfig(t *testing.T) (string, string) {
	conf := `
	{
	"cniVersion": "1.1.0",
	"name": "containerd-net",
	"plugins": [
		{
		"type": "bridge",
		"bridge": "cni0",
		"isGateway": true,
		"ipMasq": true,
		"promiscMode": true,
		"ipam": {
			"type": "host-ipam",
			"ranges": [
			[{
				"subnet": "10.88.0.0/16"
			}],
			[{
				"subnet": "2001:4860:4860::/64"
			}]
			],
			"routes": [
			{ "dst": "0.0.0.0/0" },
			{ "dst": "::/0" }
			]
		}
		},
		{
		"type": "portmap",
		"capabilities": {"portMappings": true}
		}
	]
	}`

	cniDir := t.TempDir()

	cniConfDir := path.Join(cniDir, "net.d")
	err := os.MkdirAll(cniConfDir, 0777)
	if err != nil {
		t.Fatalf("Failed to create network config dir: %v", err)
	}

	networkConfig1 := path.Join(cniConfDir, "mocknetwork1.conflist")
	f1, err := os.Create(networkConfig1)
	if err != nil {
		t.Fatalf("Failed to create network config %v: %v", f1, err)
	}

	_, err = f1.WriteString(conf)
	if err != nil {
		t.Fatalf("Failed to write network config file %v: %v", f1, err)
	}
	f1.Close()

	return cniDir, cniConfDir
}
