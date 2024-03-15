package libnetwork

func OptionDNSNoProxy() SandboxOption {
	return func(sb *Sandbox) {
		sb.config.dnsNoProxy = true
	}
}
