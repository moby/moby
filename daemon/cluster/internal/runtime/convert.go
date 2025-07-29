package runtime

import "github.com/moby/moby/api/types/swarm"

func privilegesFromAPI(privs []*swarm.RuntimePrivilege) []*PluginPrivilege {
	var out []*PluginPrivilege
	for _, p := range privs {
		out = append(out, &PluginPrivilege{
			Name:        p.Name,
			Description: p.Description,
			Value:       p.Value,
		})
	}
	return out
}

// FromAPI converts an API RuntimeSpec to a PluginSpec,
// which can be proto encoded.
func FromAPI(spec swarm.RuntimeSpec) PluginSpec {
	return PluginSpec{
		Name:       spec.Name,
		Remote:     spec.Remote,
		Privileges: privilegesFromAPI(spec.Privileges),
		Disabled:   spec.Disabled,
		Env:        spec.Env,
	}
}

func privilegesToAPI(privs []*PluginPrivilege) []*swarm.RuntimePrivilege {
	var out []*swarm.RuntimePrivilege
	for _, p := range privs {
		out = append(out, &swarm.RuntimePrivilege{
			Name:        p.Name,
			Description: p.Description,
			Value:       p.Value,
		})
	}
	return out
}

func ToAPI(spec PluginSpec) swarm.RuntimeSpec {
	return swarm.RuntimeSpec{
		Name:       spec.Name,
		Remote:     spec.Remote,
		Privileges: privilegesToAPI(spec.Privileges),
		Disabled:   spec.Disabled,
		Env:        spec.Env,
	}
}
