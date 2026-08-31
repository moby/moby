package container_test

import (
	"encoding/json"
	"testing"

	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
)

func TestMarshalConfig(t *testing.T) {
	omitted := []byte(`{"Hostname":"","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"Labels":null}`)

	bytes, err := json.Marshal(container.Config{})
	assert.NilError(t, err)
	assert.Equal(t, string(bytes), string(omitted))

	empty := container.Config{
		OnBuild: []string{},
	}

	bytes, err = json.Marshal(empty)
	assert.NilError(t, err)
	assert.Equal(t, string(bytes), string(omitted))
}
