package container

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestMarshalConfig(t *testing.T) {
	omitted := []byte(`{"Hostname":"","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":null,"Cmd":null,"Image":"","Volumes":null,"WorkingDir":"","Entrypoint":null,"Labels":null}`)

	bytes, err := json.Marshal(Config{})
	assert.NilError(t, err)
	assert.Equal(t, string(bytes), string(omitted))

	empty := Config{
		OnBuild: []string{},
	}

	bytes, err = json.Marshal(empty)
	assert.NilError(t, err)
	assert.Equal(t, string(bytes), string(omitted))
}
