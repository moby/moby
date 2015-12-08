package distribution

import (
	"reflect"
	"testing"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/docker/image"
	"github.com/docker/docker/layer"
)

func TestCreateV2Manifest(t *testing.T) {
	imgJSON := `{
    "architecture": "amd64",
    "config": {
        "AttachStderr": false,
        "AttachStdin": false,
        "AttachStdout": false,
        "Cmd": [
            "/bin/sh",
            "-c",
            "echo hi"
        ],
        "Domainname": "",
        "Entrypoint": null,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "derived=true",
            "asdf=true"
        ],
        "Hostname": "23304fc829f9",
        "Image": "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
        "Labels": {},
        "OnBuild": [],
        "OpenStdin": false,
        "StdinOnce": false,
        "Tty": false,
        "User": "",
        "Volumes": null,
        "WorkingDir": ""
    },
    "container": "e91032eb0403a61bfe085ff5a5a48e3659e5a6deae9f4d678daa2ae399d5a001",
    "container_config": {
        "AttachStderr": false,
        "AttachStdin": false,
        "AttachStdout": false,
        "Cmd": [
            "/bin/sh",
            "-c",
            "#(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]"
        ],
        "Domainname": "",
        "Entrypoint": null,
        "Env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "derived=true",
            "asdf=true"
        ],
        "Hostname": "23304fc829f9",
        "Image": "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
        "Labels": {},
        "OnBuild": [],
        "OpenStdin": false,
        "StdinOnce": false,
        "Tty": false,
        "User": "",
        "Volumes": null,
        "WorkingDir": ""
    },
    "created": "2015-11-04T23:06:32.365666163Z",
    "docker_version": "1.9.0-dev",
    "history": [
        {
            "created": "2015-10-31T22:22:54.690851953Z",
            "created_by": "/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /"
        },
        {
            "created": "2015-10-31T22:22:55.613815829Z",
            "created_by": "/bin/sh -c #(nop) CMD [\"sh\"]"
        },
        {
            "created": "2015-11-04T23:06:30.934316144Z",
            "created_by": "/bin/sh -c #(nop) ENV derived=true",
            "empty_layer": true
        },
        {
            "created": "2015-11-04T23:06:31.192097572Z",
            "created_by": "/bin/sh -c #(nop) ENV asdf=true",
            "empty_layer": true
        },
        {
            "created": "2015-11-04T23:06:32.083868454Z",
            "created_by": "/bin/sh -c dd if=/dev/zero of=/file bs=1024 count=1024"
        },
        {
            "created": "2015-11-04T23:06:32.365666163Z",
            "created_by": "/bin/sh -c #(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]",
            "empty_layer": true
        }
    ],
    "os": "linux",
    "rootfs": {
        "diff_ids": [
            "sha256:c6f988f4874bb0add23a778f753c65efe992244e148a1d2ec2a8b664fb66bbd1",
            "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
            "sha256:13f53e08df5a220ab6d13c58b2bf83a59cbdc2e04d0a3f041ddf4b0ba4112d49"
        ],
        "type": "layers"
    }
}`

	// To fill in rawJSON
	img, err := image.NewFromJSON([]byte(imgJSON))
	if err != nil {
		t.Fatalf("json decoding failed: %v", err)
	}

	fsLayers := map[layer.DiffID]schema1.FSLayer{
		layer.DiffID("sha256:c6f988f4874bb0add23a778f753c65efe992244e148a1d2ec2a8b664fb66bbd1"): {BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
		layer.DiffID("sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"): {BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		layer.DiffID("sha256:13f53e08df5a220ab6d13c58b2bf83a59cbdc2e04d0a3f041ddf4b0ba4112d49"): {BlobSum: digest.Digest("sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
	}

	manifest, err := CreateV2Manifest("testrepo", "testtag", img, fsLayers)
	if err != nil {
		t.Fatalf("CreateV2Manifest returned error: %v", err)
	}

	if manifest.Versioned.SchemaVersion != 1 {
		t.Fatal("SchemaVersion != 1")
	}
	if manifest.Name != "testrepo" {
		t.Fatal("incorrect name in manifest")
	}
	if manifest.Tag != "testtag" {
		t.Fatal("incorrect tag in manifest")
	}
	if manifest.Architecture != "amd64" {
		t.Fatal("incorrect arch in manifest")
	}

	expectedFSLayers := []schema1.FSLayer{
		{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		{BlobSum: digest.Digest("sha256:b4ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
		{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		{BlobSum: digest.Digest("sha256:86e0e091d0da6bde2456dbb48306f3956bbeb2eae1b5b9a43045843f69fe4aaa")},
		{BlobSum: digest.Digest("sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4")},
	}

	if len(manifest.FSLayers) != len(expectedFSLayers) {
		t.Fatalf("wrong number of FSLayers: %d", len(manifest.FSLayers))
	}
	if !reflect.DeepEqual(manifest.FSLayers, expectedFSLayers) {
		t.Fatal("wrong FSLayers list")
	}

	expectedV1Compatibility := []string{
		`{"architecture":"amd64","config":{"AttachStderr":false,"AttachStdin":false,"AttachStdout":false,"Cmd":["/bin/sh","-c","echo hi"],"Domainname":"","Entrypoint":null,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","derived=true","asdf=true"],"Hostname":"23304fc829f9","Image":"sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246","Labels":{},"OnBuild":[],"OpenStdin":false,"StdinOnce":false,"Tty":false,"User":"","Volumes":null,"WorkingDir":""},"container":"e91032eb0403a61bfe085ff5a5a48e3659e5a6deae9f4d678daa2ae399d5a001","container_config":{"AttachStderr":false,"AttachStdin":false,"AttachStdout":false,"Cmd":["/bin/sh","-c","#(nop) CMD [\"/bin/sh\" \"-c\" \"echo hi\"]"],"Domainname":"","Entrypoint":null,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","derived=true","asdf=true"],"Hostname":"23304fc829f9","Image":"sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246","Labels":{},"OnBuild":[],"OpenStdin":false,"StdinOnce":false,"Tty":false,"User":"","Volumes":null,"WorkingDir":""},"created":"2015-11-04T23:06:32.365666163Z","docker_version":"1.9.0-dev","id":"d728140d3fd23dfcac505954af0b2224b3579b177029eded62916579eb19ac64","os":"linux","parent":"0594e66a9830fa5ba73b66349eb221ea4beb6bac8d2148b90a0f371f8d67bcd5","throwaway":true}`,
		`{"id":"0594e66a9830fa5ba73b66349eb221ea4beb6bac8d2148b90a0f371f8d67bcd5","parent":"39bc0dbed47060dd8952b048e73744ae471fe50354d2c267d308292c53b83ce1","created":"2015-11-04T23:06:32.083868454Z","container_config":{"Cmd":["/bin/sh -c dd if=/dev/zero of=/file bs=1024 count=1024"]}}`,
		`{"id":"39bc0dbed47060dd8952b048e73744ae471fe50354d2c267d308292c53b83ce1","parent":"875d7f206c023dc979e1677567a01364074f82b61e220c9b83a4610170490381","created":"2015-11-04T23:06:31.192097572Z","container_config":{"Cmd":["/bin/sh -c #(nop) ENV asdf=true"]},"throwaway":true}`,
		`{"id":"875d7f206c023dc979e1677567a01364074f82b61e220c9b83a4610170490381","parent":"9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e","created":"2015-11-04T23:06:30.934316144Z","container_config":{"Cmd":["/bin/sh -c #(nop) ENV derived=true"]},"throwaway":true}`,
		`{"id":"9e3447ca24cb96d86ebd5960cb34d1299b07e0a0e03801d90b9969a2c187dd6e","parent":"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880","created":"2015-10-31T22:22:55.613815829Z","container_config":{"Cmd":["/bin/sh -c #(nop) CMD [\"sh\"]"]}}`,
		`{"id":"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880","created":"2015-10-31T22:22:54.690851953Z","container_config":{"Cmd":["/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /"]}}`,
	}

	if len(manifest.History) != len(expectedV1Compatibility) {
		t.Fatalf("wrong number of history entries: %d", len(manifest.History))
	}
	for i := range expectedV1Compatibility {
		if manifest.History[i].V1Compatibility != expectedV1Compatibility[i] {
			t.Fatalf("wrong V1Compatibility %d. expected:\n%s\ngot:\n%s", i, expectedV1Compatibility[i], manifest.History[i].V1Compatibility)
		}
	}
}
