package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moby/extensions"
)

// ExtensionConfig holds the opaque configuration for one extension.
type ExtensionConfig struct {
	ID     extensions.ExtensionID
	Config map[string]any
}

// ExtensionConfigs holds extension configurations in their configured order.
type ExtensionConfigs []ExtensionConfig

// UnmarshalJSON decodes extension configurations while preserving their order.
func (c *ExtensionConfigs) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		*c = nil
		return nil
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return errors.New("extension-config must be a JSON object")
	}

	configs := make(ExtensionConfigs, 0)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		id, ok := token.(string)
		if !ok {
			return errors.New("extension-config key must be a string")
		}
		extensionID := extensions.ExtensionID(id)
		if err := extensions.ValidateExtensionID(extensionID); err != nil {
			return fmt.Errorf("invalid extension-config key %q: %w", id, err)
		}

		var extensionConfig map[string]any
		if err := decoder.Decode(&extensionConfig); err != nil {
			return fmt.Errorf("invalid configuration for extension %q: %w", id, err)
		}
		configs = append(configs, ExtensionConfig{ID: extensionID, Config: extensionConfig})
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}

	*c = configs
	return nil
}

// MarshalJSON encodes extension configurations in their configured order.
func (c ExtensionConfigs) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("null"), nil
	}

	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, extensionConfig := range c {
		if i > 0 {
			buf.WriteByte(',')
		}

		if err := extensions.ValidateExtensionID(extensionConfig.ID); err != nil {
			return nil, fmt.Errorf("invalid extension-config key %q: %w", extensionConfig.ID, err)
		}
		id, err := json.Marshal(extensionConfig.ID)
		if err != nil {
			return nil, err
		}
		config, err := json.Marshal(extensionConfig.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal configuration for extension %q: %w", extensionConfig.ID, err)
		}
		buf.Write(id)
		buf.WriteByte(':')
		buf.Write(config)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
