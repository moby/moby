package runconfig

import (
	"fmt"
	"strings"

	"github.com/appc/spec/schema"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/nat"
)

func Merge(userConf, imageConf *Config) error {
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
	if len(userConf.ExposedPorts) == 0 {
		userConf.ExposedPorts = imageConf.ExposedPorts
	} else if imageConf.ExposedPorts != nil {
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}
		for port := range imageConf.ExposedPorts {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}

	if len(userConf.PortSpecs) > 0 {
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}
		ports, _, err := nat.ParsePortSpecs(userConf.PortSpecs)
		if err != nil {
			return err
		}
		for port := range ports {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
		userConf.PortSpecs = nil
	}
	if len(imageConf.PortSpecs) > 0 {
		// FIXME: I think we can safely remove this. Leaving it for now for the sake of reverse-compat paranoia.
		log.Debugf("Migrating image port specs to containter: %s", strings.Join(imageConf.PortSpecs, ", "))
		if userConf.ExposedPorts == nil {
			userConf.ExposedPorts = make(nat.PortSet)
		}

		ports, _, err := nat.ParsePortSpecs(imageConf.PortSpecs)
		if err != nil {
			return err
		}
		for port := range ports {
			if _, exists := userConf.ExposedPorts[port]; !exists {
				userConf.ExposedPorts[port] = struct{}{}
			}
		}
	}

	if len(userConf.Env) == 0 {
		userConf.Env = imageConf.Env
	} else {
		for _, imageEnv := range imageConf.Env {
			found := false
			imageEnvKey := strings.Split(imageEnv, "=")[0]
			for _, userEnv := range userConf.Env {
				userEnvKey := strings.Split(userEnv, "=")[0]
				if imageEnvKey == userEnvKey {
					found = true
				}
			}
			if !found {
				userConf.Env = append(userConf.Env, imageEnv)
			}
		}
	}

	if len(userConf.Entrypoint) == 0 {
		if len(userConf.Cmd) == 0 {
			userConf.Cmd = imageConf.Cmd
		}

		if userConf.Entrypoint == nil {
			userConf.Entrypoint = imageConf.Entrypoint
		}
	}
	if userConf.WorkingDir == "" {
		userConf.WorkingDir = imageConf.WorkingDir
	}
	if len(userConf.Volumes) == 0 {
		userConf.Volumes = imageConf.Volumes
	} else {
		for k, v := range imageConf.Volumes {
			userConf.Volumes[k] = v
		}
	}
	return nil
}

func MergeACI(userConf *Config, manifest *schema.ImageManifest) error {
	if userConf.User == "" {
		if strings.HasPrefix(manifest.App.User, "/") {
			return fmt.Errorf("ACI with user field referring to an absolute path is not yet supported by Docker")
		}
		userConf.User = manifest.App.User
	}
	if manifest.App.Group != userConf.User {
		// FIXME(ACI): Handle group correctly. For now, just do a basic partial check...
		return fmt.Errorf("Groups in ACI are not yet supported by Docker. ")
	}

	// FIXME(ACI): Read manifest.App.Isolators

	// FIXME(ACI): Do something with manifest.App.Ports

	for _, imageEnv := range manifest.App.Environment {
		found := false
		imageEnvKey := imageEnv.Name
		for _, userEnv := range userConf.Env {
			userEnvKey := strings.Split(userEnv, "=")[0]
			if imageEnvKey == userEnvKey {
				found = true
			}
		}
		if !found {
			userConf.Env = append(userConf.Env, imageEnv.Name+"="+imageEnv.Value)
		}
	}

	if len(userConf.Entrypoint) == 0 {
		userConf.Entrypoint = []string(manifest.App.Exec)
	}

	if userConf.WorkingDir == "" {
		userConf.WorkingDir = manifest.App.WorkingDirectory
	}

	// FIXME(ACI): manifest.App.MountPoints

	return nil
}
