package compression

import (
	"strconv"

	"github.com/pkg/errors"
)

const (
	attrLayerCompression = "compression"
	attrForceCompression = "force-compression"
	attrCompressionLevel = "compression-level"
)

func ParseAttributes(attrs map[string]string) (Config, error) {
	var compressionType Type
	if v, ok := attrs[attrLayerCompression]; ok {
		c, err := Parse(v)
		if err != nil {
			return Config{}, err
		}
		compressionType = c
	} else {
		compressionType = Default
	}
	compressionConfig := New(compressionType)
	if v, ok := attrs[attrForceCompression]; ok {
		var force bool
		if v == "" {
			force = true
		} else {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return Config{}, errors.Wrapf(err, "non-bool value %s specified for %s", v, attrForceCompression)
			}
			force = b
		}
		compressionConfig = compressionConfig.SetForce(force)
	}
	if v, ok := attrs[attrCompressionLevel]; ok {
		ii, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return Config{}, errors.Wrapf(err, "non-integer value %s specified for %s", v, attrCompressionLevel)
		}
		compressionConfig = compressionConfig.SetLevel(int(ii))
	}
	return compressionConfig, nil
}
