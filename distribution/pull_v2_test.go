package distribution // import "github.com/docker/docker/distribution"

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/containerd/log/logtest"
	"github.com/distribution/reference"
	"github.com/docker/distribution/manifest/schema1"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/image"
	"github.com/docker/docker/registry"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestFixManifestLayers checks that fixManifestLayers removes a duplicate
// layer, and that it makes no changes to the manifest when called a second
// time, after the duplicate is removed.
func TestFixManifestLayers(t *testing.T) {
	duplicateLayerManifest := schema1.Manifest{
		FSLayers: []schema1.FSLayer{
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		},
		History: []schema1.History{
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:07.568027497Z\",\"container\":\"fe9e5a5264a843c9292d17b736c92dd19bdb49986a8782d7389964ddaff887cc\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"cd /go/src/github.com/tonistiigi/dnsdock \\u0026\\u0026     go get -v github.com/tools/godep \\u0026\\u0026     godep restore \\u0026\\u0026     go install -ldflags \\\"-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2\\u003e/dev/null) ]]; then echo \\\"-dirty\\\"; fi`\\\" ./...\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":118430532}\n"},
		},
	}

	duplicateLayerManifestExpectedOutput := schema1.Manifest{
		FSLayers: []schema1.FSLayer{
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		},
		History: []schema1.History{
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:07.568027497Z\",\"container\":\"fe9e5a5264a843c9292d17b736c92dd19bdb49986a8782d7389964ddaff887cc\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"cd /go/src/github.com/tonistiigi/dnsdock \\u0026\\u0026     go get -v github.com/tools/godep \\u0026\\u0026     godep restore \\u0026\\u0026     go install -ldflags \\\"-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2\\u003e/dev/null) ]]; then echo \\\"-dirty\\\"; fi`\\\" ./...\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":118430532}\n"},
		},
	}

	if err := fixManifestLayers(&duplicateLayerManifest); err != nil {
		t.Fatalf("unexpected error from fixManifestLayers: %v", err)
	}

	if !reflect.DeepEqual(duplicateLayerManifest, duplicateLayerManifestExpectedOutput) {
		t.Fatal("incorrect output from fixManifestLayers on duplicate layer manifest")
	}

	// Run fixManifestLayers again and confirm that it doesn't change the
	// manifest (which no longer has duplicate layers).
	if err := fixManifestLayers(&duplicateLayerManifest); err != nil {
		t.Fatalf("unexpected error from fixManifestLayers: %v", err)
	}

	if !reflect.DeepEqual(duplicateLayerManifest, duplicateLayerManifestExpectedOutput) {
		t.Fatal("incorrect output from fixManifestLayers on duplicate layer manifest (second pass)")
	}
}

// TestFixManifestLayersBaseLayerParent makes sure that fixManifestLayers fails
// if the base layer configuration specifies a parent.
func TestFixManifestLayersBaseLayerParent(t *testing.T) {
	// TODO Windows: Fix this unit text
	if runtime.GOOS == "windows" {
		t.Skip("Needs fixing on Windows")
	}
	duplicateLayerManifest := schema1.Manifest{
		FSLayers: []schema1.FSLayer{
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		},
		History: []schema1.History{
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"parent\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"created\":\"2015-08-19T16:49:07.568027497Z\",\"container\":\"fe9e5a5264a843c9292d17b736c92dd19bdb49986a8782d7389964ddaff887cc\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"cd /go/src/github.com/tonistiigi/dnsdock \\u0026\\u0026     go get -v github.com/tools/godep \\u0026\\u0026     godep restore \\u0026\\u0026     go install -ldflags \\\"-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2\\u003e/dev/null) ]]; then echo \\\"-dirty\\\"; fi`\\\" ./...\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":118430532}\n"},
		},
	}

	if err := fixManifestLayers(&duplicateLayerManifest); err == nil || !strings.Contains(err.Error(), "invalid parent ID in the base layer of the image") {
		t.Fatalf("expected an invalid parent ID error from fixManifestLayers")
	}
}

// TestFixManifestLayersBadParent makes sure that fixManifestLayers fails
// if an image configuration specifies a parent that doesn't directly follow
// that (deduplicated) image in the image history.
func TestFixManifestLayersBadParent(t *testing.T) {
	duplicateLayerManifest := schema1.Manifest{
		FSLayers: []schema1.FSLayer{
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
			{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		},
		History: []schema1.History{
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ac3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"3b38edc92eb7c074812e217b41a6ade66888531009d6286a6f5f36a06f9841b9\",\"parent\":\"ac3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:11.368300679Z\",\"container\":\"d91be3479d5b1e84b0c00d18eea9dc777ca0ad166d51174b24283e2e6f104253\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT [\\\"/go/bin/dnsdock\\\"]\"],\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":null,\"Image\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":[\"/go/bin/dnsdock\"],\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":0}\n"},
			{V1Compatibility: "{\"id\":\"ec3025ca8cc9bcab039e193e20ec647c2da3c53a74020f2ba611601f9b2c6c02\",\"created\":\"2015-08-19T16:49:07.568027497Z\",\"container\":\"fe9e5a5264a843c9292d17b736c92dd19bdb49986a8782d7389964ddaff887cc\",\"container_config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"cd /go/src/github.com/tonistiigi/dnsdock \\u0026\\u0026     go get -v github.com/tools/godep \\u0026\\u0026     godep restore \\u0026\\u0026     go install -ldflags \\\"-X main.version `git describe --tags HEAD``if [[ -n $(command git status --porcelain --untracked-files=no 2\\u003e/dev/null) ]]; then echo \\\"-dirty\\\"; fi`\\\" ./...\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"docker_version\":\"1.6.2\",\"config\":{\"Hostname\":\"03797203757d\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/go/bin:/usr/src/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\",\"GOLANG_VERSION=1.4.1\",\"GOPATH=/go\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"e3b0ff09e647595dafee15c54cd632c900df9e82b1d4d313b1e20639a1461779\",\"Volumes\":null,\"WorkingDir\":\"/go\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[],\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":118430532}\n"},
		},
	}

	err := fixManifestLayers(&duplicateLayerManifest)
	assert.Check(t, is.ErrorContains(err, "invalid parent ID"))
}

// TestValidateManifest verifies the validateManifest function
func TestValidateManifest(t *testing.T) {
	// TODO Windows: Fix this unit text
	if runtime.GOOS == "windows" {
		t.Skip("Needs fixing on Windows")
	}
	ctx := context.TODO()
	ctx = logtest.WithT(ctx, t)

	expectedDigest, err := reference.ParseNormalizedNamed("repo@sha256:02fee8c3220ba806531f606525eceb83f4feb654f62b207191b1c9209188dedd")
	if err != nil {
		t.Fatal("could not parse reference")
	}
	expectedFSLayer0 := digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")

	// Good manifest

	goodManifestBytes, err := os.ReadFile("fixtures/validate_manifest/good_manifest")
	if err != nil {
		t.Fatal("error reading fixture:", err)
	}

	var goodSignedManifest schema1.SignedManifest
	err = json.Unmarshal(goodManifestBytes, &goodSignedManifest)
	if err != nil {
		t.Fatal("error unmarshaling manifest:", err)
	}

	verifiedManifest, err := verifySchema1Manifest(ctx, &goodSignedManifest, expectedDigest)
	if err != nil {
		t.Fatal("validateManifest failed:", err)
	}

	if verifiedManifest.FSLayers[0].BlobSum != expectedFSLayer0 {
		t.Fatal("unexpected FSLayer in good manifest")
	}

	// "Extra data" manifest

	extraDataManifestBytes, err := os.ReadFile("fixtures/validate_manifest/extra_data_manifest")
	if err != nil {
		t.Fatal("error reading fixture:", err)
	}

	var extraDataSignedManifest schema1.SignedManifest
	err = json.Unmarshal(extraDataManifestBytes, &extraDataSignedManifest)
	if err != nil {
		t.Fatal("error unmarshaling manifest:", err)
	}

	verifiedManifest, err = verifySchema1Manifest(ctx, &extraDataSignedManifest, expectedDigest)
	if err != nil {
		t.Fatal("validateManifest failed:", err)
	}

	if verifiedManifest.FSLayers[0].BlobSum != expectedFSLayer0 {
		t.Fatal("unexpected FSLayer in extra data manifest")
	}

	// Bad manifest

	badManifestBytes, err := os.ReadFile("fixtures/validate_manifest/bad_manifest")
	if err != nil {
		t.Fatal("error reading fixture:", err)
	}

	var badSignedManifest schema1.SignedManifest
	err = json.Unmarshal(badManifestBytes, &badSignedManifest)
	if err != nil {
		t.Fatal("error unmarshaling manifest:", err)
	}

	_, err = verifySchema1Manifest(ctx, &badSignedManifest, expectedDigest)
	if err == nil || !strings.HasPrefix(err.Error(), "image verification failed for digest") {
		t.Fatal("expected validateManifest to fail with digest error")
	}
}

func TestNoMatchesErr(t *testing.T) {
	err := noMatchesErr{}
	assert.Check(t, is.ErrorContains(err, "no matching manifest for "+runtime.GOOS))

	err = noMatchesErr{ocispec.Platform{
		Architecture: "arm64",
		OS:           "windows",
		OSVersion:    "10.0.17763",
		Variant:      "v8",
	}}
	assert.Check(t, is.Error(err, "no matching manifest for windows(10.0.17763)/arm64/v8 in the manifest list entries"))
}

func TestPullSchema2Config(t *testing.T) {
	ctx := context.Background()

	const imageJSON = `{
	"architecture": "amd64",
	"os": "linux",
	"config": {},
	"rootfs": {
		"type": "layers",
		"diff_ids": []
	}
}`
	expectedDigest := digest.Digest("sha256:66ad98165d38f53ee73868f82bd4eed60556ddfee824810a4062c4f777b20a5b")

	tests := []struct {
		name           string
		handler        func(callCount int, w http.ResponseWriter)
		expectError    string
		expectAttempts uint64
	}{
		{
			name: "success first time",
			handler: func(callCount int, w http.ResponseWriter) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 1,
		},
		{
			name: "500 status",
			handler: func(callCount int, w http.ResponseWriter) {
				if callCount == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 2,
		},
		{
			name: "EOF",
			handler: func(callCount int, w http.ResponseWriter) {
				if callCount == 1 {
					panic("intentional panic")
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(imageJSON))
			},
			expectAttempts: 2,
		},
		{
			name: "unauthorized",
			handler: func(callCount int, w http.ResponseWriter) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("you need to be authenticated"))
			},
			expectError:    "unauthorized: you need to be authenticated",
			expectAttempts: 1,
		},
		{
			name: "unauthorized JSON",
			handler: func(callCount int, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`					{ "errors":	[{"code": "UNAUTHORIZED", "message": "you need to be authenticated", "detail": "more detail"}]}`))
			},
			expectError:    "unauthorized: you need to be authenticated",
			expectAttempts: 1,
		},
		{
			name: "unauthorized JSON no body",
			handler: func(callCount int, w http.ResponseWriter) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
			},
			expectError:    "unauthorized: authentication required",
			expectAttempts: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var callCount atomic.Uint64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Logf("HTTP %s %s", r.Method, r.URL.Path)
				defer r.Body.Close()
				switch {
				case r.Method == "GET" && r.URL.Path == "/v2":
					w.WriteHeader(http.StatusOK)
				case r.Method == "GET" && r.URL.Path == "/v2/docker.io/library/testremotename/blobs/"+expectedDigest.String():
					tc.handler(int(callCount.Add(1)), w)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer ts.Close()

			p := testNewPuller(t, ts.URL)

			config, err := p.pullSchema2Config(ctx, expectedDigest)
			if tc.expectError == "" {
				if err != nil {
					t.Fatal(err)
				}

				_, err = image.NewFromJSON(config)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error to contain %q", tc.expectError)
				}
				if !strings.Contains(err.Error(), tc.expectError) {
					t.Fatalf("expected error=%q to contain %q", err, tc.expectError)
				}
			}

			if cc := callCount.Load(); cc != tc.expectAttempts {
				t.Fatalf("got callCount=%d but expected=%d", cc, tc.expectAttempts)
			}
		})
	}
}

func testNewPuller(t *testing.T, rawurl string) *puller {
	t.Helper()

	uri, err := url.Parse(rawurl)
	assert.NilError(t, err, "could not parse url from test server: %v", rawurl)

	n, err := reference.ParseNormalizedNamed("testremotename")
	assert.NilError(t, err)

	repoInfo := &registry.RepositoryInfo{
		Name: n,
		Index: &registrytypes.IndexInfo{
			Name: "testrepo",
		},
	}
	imagePullConfig := &ImagePullConfig{
		Config: Config{
			AuthConfig: &registrytypes.AuthConfig{
				RegistryToken: secretRegistryToken,
			},
		},
	}

	p := newPuller(registry.APIEndpoint{URL: uri}, repoInfo, imagePullConfig, nil)
	p.repo, err = newRepository(context.Background(), p.repoInfo, p.endpoint, p.config.MetaHeaders, p.config.AuthConfig, "pull")
	assert.NilError(t, err)
	return p
}
