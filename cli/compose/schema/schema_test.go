package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type dict map[string]interface{}

func TestValid(t *testing.T) {
	config := dict{
		"version": "2.1",
		"services": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	assert.NoError(t, Validate(config))
}

func TestUndefinedTopLevelOption(t *testing.T) {
	config := dict{
		"version": "2.1",
		"helicopters": dict{
			"foo": dict{
				"image": "busybox",
			},
		},
	}

	assert.Error(t, Validate(config))
}
