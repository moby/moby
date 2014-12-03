package parsers

import (
	"fmt"
	"strconv"
	"strings"
)

// FIXME: Change this not to receive default value as parameter
func ParseHost(defaultTCPAddr, defaultUnixAddr, addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = fmt.Sprintf("unix://%s", defaultUnixAddr)
	}
	addrParts := strings.Split(addr, "://")
	if len(addrParts) == 1 {
		addrParts = []string{"tcp", addrParts[0]}
	}

	switch addrParts[0] {
	case "tcp":
		return ParseTCPAddr(addrParts[1], defaultTCPAddr)
	case "unix":
		return ParseUnixAddr(addrParts[1], defaultUnixAddr)
	case "fd":
		return addr, nil
	default:
		return "", fmt.Errorf("Invalid bind address format: %s", addr)
	}
}

func ParseUnixAddr(addr string, defaultAddr string) (string, error) {
	addr = strings.TrimPrefix(addr, "unix://")
	if strings.Contains(addr, "://") {
		return "", fmt.Errorf("Invalid proto, expected unix: %s", addr)
	}
	if addr == "" {
		addr = defaultAddr
	}
	return fmt.Sprintf("unix://%s", addr), nil
}

func ParseTCPAddr(addr string, defaultAddr string) (string, error) {
	addr = strings.TrimPrefix(addr, "tcp://")
	if strings.Contains(addr, "://") || addr == "" {
		return "", fmt.Errorf("Invalid proto, expected tcp: %s", addr)
	}

	hostParts := strings.Split(addr, ":")
	if len(hostParts) != 2 {
		return "", fmt.Errorf("Invalid bind address format: %s", addr)
	}
	host := hostParts[0]
	if host == "" {
		host = defaultAddr
	}

	p, err := strconv.Atoi(hostParts[1])
	if err != nil && p == 0 {
		return "", fmt.Errorf("Invalid bind address format: %s", addr)
	}
	return fmt.Sprintf("tcp://%s:%d", host, p), nil
}

// Get a repos name and returns the right reposName + tag
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}

func PartParser(template, data string) (map[string]string, error) {
	// ip:public:private
	var (
		templateParts = strings.Split(template, ":")
		parts         = strings.Split(data, ":")
		out           = make(map[string]string, len(templateParts))
	)
	if len(parts) != len(templateParts) {
		return nil, fmt.Errorf("Invalid format to parse.  %s should match template %s", data, template)
	}

	for i, t := range templateParts {
		value := ""
		if len(parts) > i {
			value = parts[i]
		}
		out[t] = value
	}
	return out, nil
}

func ParseKeyValueOpt(opt string) (string, string, error) {
	parts := strings.SplitN(opt, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Unable to parse key/value option: %s", opt)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
