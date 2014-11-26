package runconfig

// Compare two Config struct. Do not compare the "Image" nor "Hostname" fields
// If OpenStdin is set, then it differs
func Compare(a, b *Config) bool {
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
		a.Tty != b.Tty ||
		a.Transactional != b.Transactional {
		return false
	}
	if len(a.Cmd) != len(b.Cmd) ||
		len(a.Env) != len(b.Env) ||
		len(a.PortSpecs) != len(b.PortSpecs) ||
		len(a.ExposedPorts) != len(b.ExposedPorts) ||
		len(a.Entrypoint) != len(b.Entrypoint) ||
		len(a.Volumes) != len(b.Volumes) ||
		len(a.TransactionCmds) != len(b.TransactionCmds) {
		return false
	}

	for i := 0; i < len(a.Cmd); i++ {
		if a.Cmd[i] != b.Cmd[i] {
			return false
		}
	}
	for i := 0; i < len(a.Env); i++ {
		if a.Env[i] != b.Env[i] {
			return false
		}
	}
	for i := 0; i < len(a.PortSpecs); i++ {
		if a.PortSpecs[i] != b.PortSpecs[i] {
			return false
		}
	}
	for k := range a.ExposedPorts {
		if _, exists := b.ExposedPorts[k]; !exists {
			return false
		}
	}
	for i := 0; i < len(a.Entrypoint); i++ {
		if a.Entrypoint[i] != b.Entrypoint[i] {
			return false
		}
	}
	for key := range a.Volumes {
		if _, exists := b.Volumes[key]; !exists {
			return false
		}
	}
	for i := 0; i < len(a.TransactionCmds); i++ {
		if a.TransactionCmds[i].Cmd != b.TransactionCmds[i].Cmd {
			return false
		}
		if a.TransactionCmds[i].Original != b.TransactionCmds[i].Original {
			return false
		}
		if len(a.TransactionCmds[i].Args) != len(b.TransactionCmds[i].Args) {
			return false
		}
		for j := 0; j < len(a.TransactionCmds[i].Args); j++ {
			if a.TransactionCmds[i].Args[j] !=
				b.TransactionCmds[i].Args[j] {
				return false
			}
		}
		for key := range a.TransactionCmds[i].Attributes {
			if val, exists := b.TransactionCmds[i].Attributes[key]; !exists {
				return false
			} else if a.TransactionCmds[i].Attributes[key] != val {
				return false
			}
		}
	}
	return true
}
