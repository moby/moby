package genericresource

import (
	"strconv"
	"strings"

	"github.com/moby/swarmkit/v2/api"
)

func discreteToString(d *api.GenericResource_DiscreteResourceSpec) string {
	return strconv.FormatInt(d.DiscreteResourceSpec.Value, 10)
}

// Kind returns the kind key as a string
func Kind(res *api.GenericResource) string {
	switch r := res.Resource.(type) {
	case *api.GenericResource_DiscreteResourceSpec:
		return r.DiscreteResourceSpec.Kind
	case *api.GenericResource_NamedResourceSpec:
		return r.NamedResourceSpec.Kind
	}

	return ""
}

// Value returns the value key as a string
func Value(res *api.GenericResource) string {
	switch res := res.Resource.(type) {
	case *api.GenericResource_DiscreteResourceSpec:
		return discreteToString(res)
	case *api.GenericResource_NamedResourceSpec:
		return res.NamedResourceSpec.Value
	}

	return ""
}

// EnvFormat returns the environment string version of the resource
func EnvFormat(res []*api.GenericResource, prefix string) []string {
	envs := make(map[string][]string)
	for _, v := range res {
		key := Kind(v)
		val := Value(v)
		envs[key] = append(envs[key], val)
	}

	env := make([]string, 0, len(res))
	for k, v := range envs {
		k = strings.ToUpper(prefix + "_" + k)
		env = append(env, k+"="+strings.Join(v, ","))
	}

	return env
}
