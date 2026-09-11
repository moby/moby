package system

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moby/moby/api/types/build"
	"gotest.tools/v3/assert"
)

func TestPingBuilderVersion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		version build.BuilderVersion
	}{
		{name: "classic builder", version: build.BuilderV1},
		{name: "buildkit", version: build.BuilderBuildKit},
		{name: "unspecified"},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				t.Parallel()

				s := &systemRouter{
					backend: &builderVersionBackend{version: tc.version},
				}
				req := httptest.NewRequestWithContext(t.Context(), method, "/_ping", nil)
				rec := httptest.NewRecorder()

				assert.NilError(t, s.pingHandler(t.Context(), rec, req, nil))
				assert.Equal(t, rec.Code, http.StatusOK)
				assert.Equal(t, rec.Header().Get("Builder-Version"), string(tc.version))
				_, present := rec.Header()["Builder-Version"]
				assert.Equal(t, present, tc.version != "")
			})
		}
	}
}

type builderVersionBackend struct {
	Backend
	version build.BuilderVersion
}

func (b *builderVersionBackend) BuilderVersion() build.BuilderVersion {
	return b.version
}
