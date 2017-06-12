package genericresource

import (
	"strconv"
	"strings"

	"github.com/docker/swarmkit/api"
)

func discreteToString(d *api.GenericResource_Discrete) string {
	return strconv.FormatInt(d.Discrete.Value, 10)
}

// Kind returns the kind key as a string
func Kind(res *api.GenericResource) string {
	switch r := res.Resource.(type) {
	case *api.GenericResource_Discrete:
		return r.Discrete.Kind
	case *api.GenericResource_Str:
		return r.Str.Kind
	}

	return ""
}

// Value returns the value key as a string
func Value(res *api.GenericResource) string {
	switch res := res.Resource.(type) {
	case *api.GenericResource_Discrete:
		return discreteToString(res)
	case *api.GenericResource_Str:
		return res.Str.Value
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
