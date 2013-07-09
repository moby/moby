package docker

// Compare two Config struct. Do not compare the "Image" nor "Hostname" fields
// If OpenStdin is set, then it differs
func CompareConfig(a, b *Config) bool {
	if a == nil || b == nil ||
		a.OpenStdin || b.OpenStdin {
		return false
	}
	if a.AttachStdout != b.AttachStdout ||
		a.AttachStderr != b.AttachStderr ||
		a.User != b.User ||
		a.Memory != b.Memory ||
		a.MemorySwap != b.MemorySwap ||
		a.CpuShares != b.CpuShares ||
		a.OpenStdin != b.OpenStdin ||
		a.Tty != b.Tty {
		return false
	}
	if len(a.Cmd) != len(b.Cmd) ||
		len(a.Dns) != len(b.Dns) ||
		len(a.Env) != len(b.Env) ||
		len(a.PortSpecs) != len(b.PortSpecs) ||
		len(a.Entrypoint) != len(b.Entrypoint) {
		return false
	}

	for i := 0; i < len(a.Cmd); i++ {
		if a.Cmd[i] != b.Cmd[i] {
			return false
		}
	}
	for i := 0; i < len(a.Dns); i++ {
		if a.Dns[i] != b.Dns[i] {
			return false
		}
	}
	for key, _ := range a.Env {
		if a.Env[key] != b.Env[key] {
			return false
		}
	}
	for i := 0; i < len(a.PortSpecs); i++ {
		if a.PortSpecs[i] != b.PortSpecs[i] {
			return false
		}
	}
	for i := 0; i < len(a.Entrypoint); i++ {
		if a.Entrypoint[i] != b.Entrypoint[i] {
			return false
		}
	}
	return true
}

func MergeConfig(userConf, imageConf *Config) {
	if userConf.User == "" {
		userConf.User = imageConf.User
	}
	if userConf.Memory == 0 {
		userConf.Memory = imageConf.Memory
	}
	if userConf.MemorySwap == 0 {
		userConf.MemorySwap = imageConf.MemorySwap
	}
	if userConf.CpuShares == 0 {
		userConf.CpuShares = imageConf.CpuShares
	}
	if userConf.PortSpecs == nil || len(userConf.PortSpecs) == 0 {
		userConf.PortSpecs = imageConf.PortSpecs
	}
	if !userConf.Tty {
		userConf.Tty = imageConf.Tty
	}
	if !userConf.OpenStdin {
		userConf.OpenStdin = imageConf.OpenStdin
	}
	if !userConf.StdinOnce {
		userConf.StdinOnce = imageConf.StdinOnce
	}
	if userConf.Env == nil || len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	}
	if userConf.Cmd == nil || len(userConf.Cmd) == 0 {
		userConf.Cmd = imageConf.Cmd
	}
	if userConf.Dns == nil || len(userConf.Dns) == 0 {
		userConf.Dns = imageConf.Dns
	}
	if userConf.Entrypoint == nil || len(userConf.Entrypoint) == 0 {
		userConf.Entrypoint = imageConf.Entrypoint
	}
	if userConf.Volumes == nil || len(userConf.Volumes) == 0 {
		userConf.Volumes = imageConf.Volumes
	}
}
