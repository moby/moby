package container

import (
	"os"
	"slices"
	"testing"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"go.yaml.in/yaml/v3"
	"gotest.tools/v3/assert"
)

func TestContainerSummarySchema(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"openapi", "swagger"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			type property struct {
				Type     string `yaml:"type"`
				Nullable bool   `yaml:"x-nullable"`
			}
			type summarySchema struct {
				Type       string              `yaml:"type"`
				Required   []string            `yaml:"required"`
				Properties map[string]property `yaml:"properties"`
			}
			var document struct {
				Components struct {
					Schemas map[string]summarySchema `yaml:"schemas"`
				} `yaml:"components"`
				Definitions map[string]summarySchema `yaml:"definitions"`
			}
			data, err := os.ReadFile("../../api/" + name + ".yaml")
			assert.NilError(t, err)
			assert.NilError(t, yaml.Unmarshal(data, &document))
			schemas := document.Definitions
			if name == "openapi" {
				schemas = document.Components.Schemas
			}
			schema, ok := schemas["ContainerSummary"]
			assert.Assert(t, ok)
			assert.Equal(t, schema.Type, "object")

			// Compare sets without depending on the order in the specifications.
			required := []string{
				"Id", "Names", "Image", "ImageID", "Command", "Created", "Ports",
				"Labels", "State", "Status", "HostConfig", "NetworkSettings", "Mounts",
			}
			slices.Sort(required)
			slices.Sort(schema.Required)
			assert.DeepEqual(t, schema.Required, required)
			for _, field := range []string{"SizeRw", "SizeRootFs", "Health", "ImageManifestDescriptor"} {
				_, ok := schema.Properties[field]
				assert.Check(t, ok, "missing conditional property %s", field)
				assert.Check(t, !slices.Contains(schema.Required, field), "%s must remain optional", field)
			}
			assert.Equal(t, schema.Properties["Labels"].Type, "object")
			assert.Assert(t, schema.Properties["Labels"].Nullable)

			// Validate key presence independently of property values and references.
			// The view tests cover the values emitted by the daemon. In particular,
			// a present null Labels value must not be confused with an absent key.
			presence := &spec.Schema{SchemaProps: spec.SchemaProps{
				Type:     spec.StringOrArray{schema.Type},
				Required: schema.Required,
			}}
			fields := make(map[string]any, len(required))
			for _, field := range required {
				fields[field] = nil
			}
			assert.NilError(t, validate.AgainstSchema(presence, fields, strfmt.Default))
			for _, field := range required {
				t.Run("missing "+field, func(t *testing.T) {
					delete(fields, field)
					defer func() { fields[field] = nil }()
					err := validate.AgainstSchema(presence, fields, strfmt.Default)
					assert.ErrorContains(t, err, field+" in body is required")
				})
			}
		})
	}
}
