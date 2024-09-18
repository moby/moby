package specialimage

import (
	"io"
	"os"
	"path/filepath"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// EmptyFS builds an image with an empty rootfs.
// Layout: Legacy Docker Archive
// See https://github.com/docker/docker/pull/5262
// and also https://github.com/docker/docker/issues/4242
func EmptyFS(dir string) (*ocispec.Index, error) {
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`[{"Config":"11f64303f0f7ffdc71f001788132bca5346831939a956e3e975c93267d89a16d.json","RepoTags":["emptyfs:latest"],"Layers":["511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158/layer.tar"]}]`), 0o644); err != nil {
		return nil, err
	}

	if err := os.Mkdir(filepath.Join(dir, "blobs"), 0o755); err != nil {
		return nil, err
	}

	blobsDir := filepath.Join(dir, "511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158")
	if err := os.Mkdir(blobsDir, 0o755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(`1.0`), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "repositories"), []byte(`{"emptyfs":{"latest":"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158"}}`), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "11f64303f0f7ffdc71f001788132bca5346831939a956e3e975c93267d89a16d.json"), []byte(`{"architecture":"x86_64","comment":"Imported from -","container_config":{"Hostname":"","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":null},"created":"2013-06-13T14:03:50.821769-07:00","docker_version":"0.4.0","history":[{"created":"2013-06-13T14:03:50.821769-07:00","comment":"Imported from -"}],"rootfs":{"type":"layers","diff_ids":["sha256:84ff92691f909a05b224e1c56abb4864f01b4f8e3c854e4bb4c7baf1d3f6d652"]}}`), 0o644); err != nil {
		return nil, err
	}

	if err := os.WriteFile(filepath.Join(blobsDir, "json"), []byte(`{"id":"511136ea3c5a64f264b78b5433614aec563103b4d4702f3ba7d4d2698e22c158","comment":"Imported from -","created":"2013-06-13T14:03:50.821769-07:00","container_config":{"Hostname":"","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"NetworkDisabled":false,"OnBuild":null},"docker_version":"0.4.0","architecture":"x86_64","Size":0}`+"\n"), 0o644); err != nil {
		return nil, err
	}

	layerFile, err := os.OpenFile(filepath.Join(blobsDir, "layer.tar"), os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	defer layerFile.Close()

	// 10240 NUL bytes is a valid empty tar archive.
	_, err = io.Copy(layerFile, io.LimitReader(zeroReader{}, 10240))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

type zeroReader struct{}

func (_ zeroReader) Read(p []byte) (n int, err error) {
	l := len(p)
	for idx := 0; idx < l; idx++ {
		p[idx] = 0
	}
	return l, nil
}
