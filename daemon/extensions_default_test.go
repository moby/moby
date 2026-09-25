package daemon

import (
	"testing"

	"github.com/moby/extensions"
	servicev0 "github.com/moby/extensions/extpoints/service/v0"
	"github.com/moby/extensions/host"
	"github.com/moby/moby/v2/daemon/config"
	"github.com/moby/moby/v2/daemon/internal/jobs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// declaredIDs collects the extension IDs a builtinExtensions result declares.
// Declaration is nil-daemon safe on every builtin: none dereferences the
// daemon before serving.
func declaredIDs(t *testing.T, cfg *config.Config) []extensions.ExtensionID {
	t.Helper()
	exts, err := builtinExtensions(cfg, nil)
	assert.NilError(t, err)
	ids := make([]extensions.ExtensionID, 0, len(exts))
	for _, ext := range exts {
		ids = append(ids, ext.Declaration().ID)
	}
	return ids
}

func TestBuiltinExtensionsEnableExtensions(t *testing.T) {
	// The built-in list is unconditional: opting in changes the provider
	// policy, not the list, so both configs register the same extensions.
	all := declaredIDs(t, &config.Config{})

	cfg := &config.Config{}
	cfg.EnableExtensions = []string{jobs.ExtensionID, jobs.ExtensionID}
	assert.Check(t, is.DeepEqual(declaredIDs(t, cfg), all))

	// Every ID that enable-extensions accepts maps to a real extension in
	// the list; a key without one would validate an ID that enables nothing.
	for id := range optionalBuiltins {
		assert.Check(t, is.Contains(all, extensions.ExtensionID(id)))
	}

	// An unknown ID fails startup loudly, with the available IDs listed.
	cfg = &config.Config{}
	cfg.EnableExtensions = []string{"org.example.bogus.v1"}
	_, err := builtinExtensions(cfg, nil)
	assert.Check(t, is.ErrorContains(err, `unknown built-in extension "org.example.bogus.v1"`))
	assert.Check(t, is.ErrorContains(err, jobs.ExtensionID))
}

func TestBuiltinPolicyGatesOptionalBuiltins(t *testing.T) {
	builtinJobs := extensions.ExtensionIdentity{
		ID:     jobs.ExtensionID,
		Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin},
	}
	// The service publication decision is the one that makes the host skip
	// a fully dropped extension, so probe the policy with the real point.
	point := servicev0.Point.ID()

	// An optional built-in that was not opted into has every point use
	// dropped, which makes the host skip the extension entirely.
	policy := builtinPolicy(&config.Config{})
	assert.Check(t, is.Equal(policy.Decide(builtinJobs, point), host.Drop()))

	// Opting in admits it like any always-on extension.
	cfg := &config.Config{}
	cfg.EnableExtensions = []string{jobs.ExtensionID}
	policy = builtinPolicy(cfg)
	assert.Check(t, is.Equal(policy.Decide(builtinJobs, point), host.Allow()))

	// The gate is scoped to built-ins: an extension loaded from
	// --extension-dir is enabled by its presence there, even when it
	// declares an optional built-in's ID.
	executableJobs := extensions.ExtensionIdentity{
		ID:     jobs.ExtensionID,
		Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginExecutable},
	}
	policy = builtinPolicy(&config.Config{})
	assert.Check(t, is.Equal(policy.Decide(executableJobs, point), host.Allow()))

	// Extensions that are not optional built-ins are never gated here,
	// whatever the enable list says.
	builtinRuntime := extensions.ExtensionIdentity{
		ID:     runtimeExtensionID,
		Origin: extensions.ExtensionOrigin{Kind: extensions.ExtensionOriginBuiltin},
	}
	assert.Check(t, is.Equal(policy.Decide(builtinRuntime, point), host.Allow()))
}
