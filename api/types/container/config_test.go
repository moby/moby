package container_test

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestMarshalConfig(t *testing.T) {
	omitted := []byte(`{"Hostname":"","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"Labels":null}`)

	bytes, err := json.Marshal(container.Config{})
	checkNoError(t, err)
	checkEqual(t, string(bytes), string(omitted))

	empty := container.Config{
		OnBuild: []string{},
	}

	bytes, err = json.Marshal(empty)
	checkNoError(t, err)
	checkEqual(t, string(bytes), string(omitted))
}
