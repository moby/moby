package interpolation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var defaults = map[string]string{
	"USER": "jenny",
	"FOO":  "bar",
}

func defaultMapping(name string) (string, bool) {
	val, ok := defaults[name]
	return val, ok
}

func TestInterpolate(t *testing.T) {
	services := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image":   "example:${USER}",
			"volumes": []interface{}{"$FOO:/target"},
			"logging": map[string]interface{}{
				"driver": "${FOO}",
				"options": map[string]interface{}{
					"user": "$USER",
				},
			},
		},
	}
	expected := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image":   "example:jenny",
			"volumes": []interface{}{"bar:/target"},
			"logging": map[string]interface{}{
				"driver": "bar",
				"options": map[string]interface{}{
					"user": "jenny",
				},
			},
		},
	}
	result, err := Interpolate(services, "service", defaultMapping)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestInvalidInterpolation(t *testing.T) {
	services := map[string]interface{}{
		"servicea": map[string]interface{}{
			"image": "${",
		},
	}
	_, err := Interpolate(services, "service", defaultMapping)
	assert.EqualError(t, err, `Invalid interpolation format for "image" option in service "servicea": "${". You may need to escape any $ with another $.`)
}
