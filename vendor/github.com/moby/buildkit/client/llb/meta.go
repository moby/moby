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
)

type contextKeyT string

var (
	keyArgs      = contextKeyT("llb.exec.args")
	keyDir       = contextKeyT("llb.exec.dir")
	keyEnv       = contextKeyT("llb.exec.env")
	keyUser      = contextKeyT("llb.exec.user")
	keyHostname  = contextKeyT("llb.exec.hostname")
	keyExtraHost = contextKeyT("llb.exec.extrahost")
	keyPlatform  = contextKeyT("llb.platform")
	keyNetwork   = contextKeyT("llb.network")
	keySecurity  = contextKeyT("llb.security")
)

func AddEnvf(key, value string, v ...interface{}) StateOption {
	return addEnvf(key, value, true, v...)
}

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

func Dir(str string) StateOption {
	return dirf(str, false)
}

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
					return nil, err
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

func User(str string) StateOption {
	return func(s State) State {
		return s.WithValue(keyUser, str)
	}
}

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
