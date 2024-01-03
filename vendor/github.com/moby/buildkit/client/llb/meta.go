package llb

import (
	"context"
	"fmt"
	"net"
	"path"

	"github.com/containerd/containerd/platforms"
	"github.com/google/shlex"
	"github.com/moby/buildkit/solver/pb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

type contextKeyT string

var (
	keyArgs         = contextKeyT("llb.exec.args")
	keyDir          = contextKeyT("llb.exec.dir")
	keyEnv          = contextKeyT("llb.exec.env")
	keyExtraHost    = contextKeyT("llb.exec.extrahost")
	keyHostname     = contextKeyT("llb.exec.hostname")
	keyUlimit       = contextKeyT("llb.exec.ulimit")
	keyCgroupParent = contextKeyT("llb.exec.cgroup.parent")
	keyUser         = contextKeyT("llb.exec.user")

	keyPlatform = contextKeyT("llb.platform")
	keyNetwork  = contextKeyT("llb.network")
	keySecurity = contextKeyT("llb.security")
)

// AddEnvf is the same as [AddEnv] but allows for a format string.
// This is the equivalent of `[State.AddEnvf]`
func AddEnvf(key, value string, v ...interface{}) StateOption {
	return addEnvf(key, value, true, v...)
}

// AddEnv returns a [StateOption] whichs adds an environment variable to the state.
// Use this with [State.With] to create a new state with the environment variable set.
// This is the equivalent of `[State.AddEnv]`
func AddEnv(key, value string) StateOption {
	return addEnvf(key, value, false)
}

func addEnvf(key, value string, replace bool, v ...interface{}) StateOption {
	if replace {
		value = fmt.Sprintf(value, v...)
	}
	return func(s State) State {
		return s.withValue(keyEnv, func(ctx context.Context, c *Constraints) (interface{}, error) {
			env, err := getEnv(s)(ctx, c)
			if err != nil {
				return nil, err
			}
			return env.AddOrReplace(key, value), nil
		})
	}
}

// Dir returns a [StateOption] sets the working directory for the state which will be used to resolve
// relative paths as well as the working directory for [State.Run].
// See [State.With] for where to use this.
func Dir(str string) StateOption {
	return dirf(str, false)
}

// Dirf is the same as [Dir] but allows for a format string.
func Dirf(str string, v ...interface{}) StateOption {
	return dirf(str, true, v...)
}

func dirf(value string, replace bool, v ...interface{}) StateOption {
	if replace {
		value = fmt.Sprintf(value, v...)
	}
	return func(s State) State {
		return s.withValue(keyDir, func(ctx context.Context, c *Constraints) (interface{}, error) {
			if !path.IsAbs(value) {
				prev, err := getDir(s)(ctx, c)
				if err != nil {
					return nil, errors.Wrap(err, "getting dir from state")
				}
				if prev == "" {
					prev = "/"
				}
				value = path.Join(prev, value)
			}
			return value, nil
		})
	}
}

// User returns a [StateOption] which sets the user for the state which will be used by [State.Run].
// This is the equivalent of [State.User]
// See [State.With] for where to use this.
func User(str string) StateOption {
	return func(s State) State {
		return s.WithValue(keyUser, str)
	}
}

// Reset returns a [StateOption] which creates a new [State] with just the
// output of the current [State] and the provided [State] is set as the parent.
// This is the equivalent of [State.Reset]
func Reset(other State) StateOption {
	return func(s State) State {
		s = NewState(s.Output())
		s.prev = &other
		return s
	}
}

func getEnv(s State) func(context.Context, *Constraints) (EnvList, error) {
	return func(ctx context.Context, c *Constraints) (EnvList, error) {
		v, err := s.getValue(keyEnv)(ctx, c)
		if err != nil {
			return nil, err
		}
		if v != nil {
			return v.(EnvList), nil
		}
		return EnvList{}, nil
	}
}

func getDir(s State) func(context.Context, *Constraints) (string, error) {
	return func(ctx context.Context, c *Constraints) (string, error) {
		v, err := s.getValue(keyDir)(ctx, c)
		if err != nil {
			return "", err
		}
		if v != nil {
			return v.(string), nil
		}
		return "", nil
	}
}

func getArgs(s State) func(context.Context, *Constraints) ([]string, error) {
	return func(ctx context.Context, c *Constraints) ([]string, error) {
		v, err := s.getValue(keyArgs)(ctx, c)
		if err != nil {
			return nil, err
		}
		if v != nil {
			return v.([]string), nil
		}
		return nil, nil
	}
}

func getUser(s State) func(context.Context, *Constraints) (string, error) {
	return func(ctx context.Context, c *Constraints) (string, error) {
		v, err := s.getValue(keyUser)(ctx, c)
		if err != nil {
			return "", err
		}
		if v != nil {
			return v.(string), nil
		}
		return "", nil
	}
}

// Hostname returns a [StateOption] which sets the hostname used for containers created by [State.Run].
// This is the equivalent of [State.Hostname]
// See [State.With] for where to use this.
func Hostname(str string) StateOption {
	return func(s State) State {
		return s.WithValue(keyHostname, str)
	}
}

func getHostname(s State) func(context.Context, *Constraints) (string, error) {
	return func(ctx context.Context, c *Constraints) (string, error) {
		v, err := s.getValue(keyHostname)(ctx, c)
		if err != nil {
			return "", err
		}
		if v != nil {
			return v.(string), nil
		}
		return "", nil
	}
}

func args(args ...string) StateOption {
	return func(s State) State {
		return s.WithValue(keyArgs, args)
	}
}

func shlexf(str string, replace bool, v ...interface{}) StateOption {
	if replace {
		str = fmt.Sprintf(str, v...)
	}
	return func(s State) State {
		arg, err := shlex.Split(str)
		if err != nil { //nolint
			// TODO: handle error
		}
		return args(arg...)(s)
	}
}

func platform(p ocispecs.Platform) StateOption {
	return func(s State) State {
		return s.WithValue(keyPlatform, platforms.Normalize(p))
	}
}

func getPlatform(s State) func(context.Context, *Constraints) (*ocispecs.Platform, error) {
	return func(ctx context.Context, c *Constraints) (*ocispecs.Platform, error) {
		v, err := s.getValue(keyPlatform)(ctx, c)
		if err != nil {
			return nil, err
		}
		if v != nil {
			p := v.(ocispecs.Platform)
			return &p, nil
		}
		return nil, nil
	}
}

func extraHost(host string, ip net.IP) StateOption {
	return func(s State) State {
		return s.withValue(keyExtraHost, func(ctx context.Context, c *Constraints) (interface{}, error) {
			v, err := getExtraHosts(s)(ctx, c)
			if err != nil {
				return nil, err
			}
			return append(v, HostIP{Host: host, IP: ip}), nil
		})
	}
}

func getExtraHosts(s State) func(context.Context, *Constraints) ([]HostIP, error) {
	return func(ctx context.Context, c *Constraints) ([]HostIP, error) {
		v, err := s.getValue(keyExtraHost)(ctx, c)
		if err != nil {
			return nil, err
		}
		if v != nil {
			return v.([]HostIP), nil
		}
		return nil, nil
	}
}

type HostIP struct {
	Host string
	IP   net.IP
}

func ulimit(name UlimitName, soft int64, hard int64) StateOption {
	return func(s State) State {
		return s.withValue(keyUlimit, func(ctx context.Context, c *Constraints) (interface{}, error) {
			v, err := getUlimit(s)(ctx, c)
			if err != nil {
				return nil, err
			}
			return append(v, pb.Ulimit{
				Name: string(name),
				Soft: soft,
				Hard: hard,
			}), nil
		})
	}
}

func getUlimit(s State) func(context.Context, *Constraints) ([]pb.Ulimit, error) {
	return func(ctx context.Context, c *Constraints) ([]pb.Ulimit, error) {
		v, err := s.getValue(keyUlimit)(ctx, c)
		if err != nil {
			return nil, err
		}
		if v != nil {
			return v.([]pb.Ulimit), nil
		}
		return nil, nil
	}
}

func cgroupParent(cp string) StateOption {
	return func(s State) State {
		return s.WithValue(keyCgroupParent, cp)
	}
}

func getCgroupParent(s State) func(context.Context, *Constraints) (string, error) {
	return func(ctx context.Context, c *Constraints) (string, error) {
		v, err := s.getValue(keyCgroupParent)(ctx, c)
		if err != nil {
			return "", err
		}
		if v != nil {
			return v.(string), nil
		}
		return "", nil
	}
}

// Network returns a [StateOption] which sets the network mode used for containers created by [State.Run].
// This is the equivalent of [State.Network]
// See [State.With] for where to use this.
func Network(v pb.NetMode) StateOption {
	return func(s State) State {
		return s.WithValue(keyNetwork, v)
	}
}
func getNetwork(s State) func(context.Context, *Constraints) (pb.NetMode, error) {
	return func(ctx context.Context, c *Constraints) (pb.NetMode, error) {
		v, err := s.getValue(keyNetwork)(ctx, c)
		if err != nil {
			return 0, err
		}
		if v != nil {
			n := v.(pb.NetMode)
			return n, nil
		}
		return NetModeSandbox, nil
	}
}

// Security returns a [StateOption] which sets the security mode used for containers created by [State.Run].
// This is the equivalent of [State.Security]
// See [State.With] for where to use this.
func Security(v pb.SecurityMode) StateOption {
	return func(s State) State {
		return s.WithValue(keySecurity, v)
	}
}
func getSecurity(s State) func(context.Context, *Constraints) (pb.SecurityMode, error) {
	return func(ctx context.Context, c *Constraints) (pb.SecurityMode, error) {
		v, err := s.getValue(keySecurity)(ctx, c)
		if err != nil {
			return 0, err
		}
		if v != nil {
			n := v.(pb.SecurityMode)
			return n, nil
		}
		return SecurityModeSandbox, nil
	}
}

type EnvList []KeyValue

type KeyValue struct {
	key   string
	value string
}

func (e EnvList) AddOrReplace(k, v string) EnvList {
	e = e.Delete(k)
	e = append(e, KeyValue{key: k, value: v})
	return e
}

func (e EnvList) SetDefault(k, v string) EnvList {
	if _, ok := e.Get(k); !ok {
		e = append(e, KeyValue{key: k, value: v})
	}
	return e
}

func (e EnvList) Delete(k string) EnvList {
	e = append([]KeyValue(nil), e...)
	if i, ok := e.Index(k); ok {
		return append(e[:i], e[i+1:]...)
	}
	return e
}

func (e EnvList) Get(k string) (string, bool) {
	if index, ok := e.Index(k); ok {
		return e[index].value, true
	}
	return "", false
}

func (e EnvList) Index(k string) (int, bool) {
	for i, kv := range e {
		if kv.key == k {
			return i, true
		}
	}
	return -1, false
}

func (e EnvList) ToArray() []string {
	out := make([]string, 0, len(e))
	for _, kv := range e {
		out = append(out, kv.key+"="+kv.value)
	}
	return out
}
