package config

import (
	"secsys/gout-transformation/pkg/transstruct"
	"testing"

	"gotest.tools/v3/fs"
)

func FuzzTestBuilderGC(XVl []byte) int {
	t := &testing.T{}
	_ = t
	transstruct.SetFuzzData(XVl)
	FDG_FuzzGlobal()

	tempFile := fs.NewFile(t, "config", fs.WithContent(transstruct.GetString(`{
  "builder": {
    "gc": {
      "enabled": true,
      "policy": [
        {"keepStorage": "10GB", "filter": ["unused-for=2200h"]},
        {"keepStorage": "50GB", "filter": {"unused-for": {"3300h": true}}},
        {"keepStorage": "100GB", "all": true}
      ]
    }
  }
}`)))
	defer tempFile.Remove()
	configFile := tempFile.Path()

	MergeDaemonConfigurations(&Config{}, nil, configFile)
	return 1
}

func FDG_FuzzGlobal() {

}
