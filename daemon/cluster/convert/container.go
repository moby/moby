package convert

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/moby/api/types/container"
	mounttypes "github.com/moby/moby/api/types/mount"
	types "github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/internal/sliceutil"
	swarmapi "github.com/moby/swarmkit/v2/api"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func containerSpecFromGRPC(c *swarmapi.ContainerSpec) *types.ContainerSpec {
	if c == nil {
		return nil
	}
	containerSpec := &types.ContainerSpec{
		Image:          c.Image,
		Labels:         c.Labels,
		Command:        c.Command,
		Args:           c.Args,
		Hostname:       c.Hostname,
		Env:            c.Env,
		Dir:            c.Dir,
		User:           c.User,
		Groups:         c.Groups,
		StopSignal:     c.StopSignal,
		TTY:            c.Tty,
		OpenStdin:      c.OpenStdin,
		ReadOnly:       c.ReadOnly,
		Hosts:          c.Hosts,
		Secrets:        secretReferencesFromGRPC(c.Secrets),
		Configs:        configReferencesFromGRPC(c.Configs),
		Isolation:      IsolationFromGRPC(c.Isolation),
		Init:           initFromGRPC(c.Init),
		Sysctls:        c.Sysctls,
		CapabilityAdd:  c.CapabilityAdd,
		CapabilityDrop: c.CapabilityDrop,
		Ulimits:        ulimitsFromGRPC(c.Ulimits),
		OomScoreAdj:    c.OomScoreAdj,
	}

	if c.DnsConfig != nil {
		containerSpec.DNSConfig = &types.DNSConfig{
			Nameservers: sliceutil.Map(c.DnsConfig.Nameservers, func(s string) netip.Addr { a, _ := netip.ParseAddr(s); return a }),
			Search:      c.DnsConfig.Search,
			Options:     c.DnsConfig.Options,
		}
	}

	// Privileges
	if c.Privileges != nil {
		containerSpec.Privileges = &types.Privileges{}

		if c.Privileges.CredentialSpec != nil {
			containerSpec.Privileges.CredentialSpec = credentialSpecFromGRPC(c.Privileges.CredentialSpec)
		}

		if c.Privileges.SelinuxContext != nil {
			containerSpec.Privileges.SELinuxContext = &types.SELinuxContext{
				Disable: c.Privileges.SelinuxContext.Disable,
				User:    c.Privileges.SelinuxContext.User,
				Type:    c.Privileges.SelinuxContext.Type,
				Role:    c.Privileges.SelinuxContext.Role,
				Level:   c.Privileges.SelinuxContext.Level,
			}
		}

		if c.Privileges.Seccomp != nil {
			containerSpec.Privileges.Seccomp = &types.SeccompOpts{
				Profile: c.Privileges.Seccomp.Profile,
			}

			switch c.Privileges.Seccomp.Mode {
			case swarmapi.Privileges_SeccompOpts_DEFAULT:
				containerSpec.Privileges.Seccomp.Mode = types.SeccompModeDefault
			case swarmapi.Privileges_SeccompOpts_UNCONFINED:
				containerSpec.Privileges.Seccomp.Mode = types.SeccompModeUnconfined
			case swarmapi.Privileges_SeccompOpts_CUSTOM:
				containerSpec.Privileges.Seccomp.Mode = types.SeccompModeCustom
			}
		}

		if c.Privileges.Apparmor != nil {
			containerSpec.Privileges.AppArmor = &types.AppArmorOpts{}

			switch c.Privileges.Apparmor.Mode {
			case swarmapi.Privileges_AppArmorOpts_DEFAULT:
				containerSpec.Privileges.AppArmor.Mode = types.AppArmorModeDefault
			case swarmapi.Privileges_AppArmorOpts_DISABLED:
				containerSpec.Privileges.AppArmor.Mode = types.AppArmorModeDisabled
			}
		}

		containerSpec.Privileges.NoNewPrivileges = c.Privileges.NoNewPrivileges
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := mounttypes.Mount{
			Target:   m.Target,
			Source:   m.Source,
			Type:     mounttypes.Type(strings.ToLower(swarmapi.Mount_Type_name[int32(m.Type)])),
			ReadOnly: m.Readonly,
		}

		if m.BindOptions != nil {
			mount.BindOptions = &mounttypes.BindOptions{
				Propagation:            mounttypes.Propagation(strings.ToLower(swarmapi.Mount_BindOptions_Propagation_name[int32(m.BindOptions.Propagation)])),
				NonRecursive:           m.BindOptions.Nonrecursive,
				CreateMountpoint:       m.BindOptions.Createmountpoint,
				ReadOnlyNonRecursive:   m.BindOptions.Readonlynonrecursive,
				ReadOnlyForceRecursive: m.BindOptions.Readonlyforcerecursive,
			}
		}

		if m.VolumeOptions != nil {
			mount.VolumeOptions = &mounttypes.VolumeOptions{
				NoCopy:  m.VolumeOptions.Nocopy,
				Labels:  m.VolumeOptions.Labels,
				Subpath: m.VolumeOptions.Subpath,
			}
			if m.VolumeOptions.DriverConfig != nil {
				mount.VolumeOptions.DriverConfig = &mounttypes.Driver{
					Name:    m.VolumeOptions.DriverConfig.Name,
					Options: m.VolumeOptions.DriverConfig.Options,
				}
			}
		}

		if m.TmpfsOptions != nil {
			mount.TmpfsOptions = &mounttypes.TmpfsOptions{
				SizeBytes: m.TmpfsOptions.SizeBytes,
				Mode:      os.FileMode(m.TmpfsOptions.Mode),
				Options:   tmpfsOptionsFromGRPC(m.TmpfsOptions.Options),
			}
		}
		containerSpec.Mounts = append(containerSpec.Mounts, mount)
	}

	if c.StopGracePeriod != nil {
		grace := c.StopGracePeriod.AsDuration()
		containerSpec.StopGracePeriod = &grace
	}

	if c.Healthcheck != nil {
		containerSpec.Healthcheck = healthConfigFromGRPC(c.Healthcheck)
	}

	return containerSpec
}

func initFromGRPC(v *wrapperspb.BoolValue) *bool {
	if v == nil {
		return nil
	}
	value := v.GetValue()
	return &value
}

func initToGRPC(v *bool) *wrapperspb.BoolValue {
	if v == nil {
		return nil
	}
	return wrapperspb.Bool(*v)
}

func secretReferencesToGRPC(sr []*types.SecretReference) []*swarmapi.SecretReference {
	refs := make([]*swarmapi.SecretReference, 0, len(sr))
	for _, s := range sr {
		ref := &swarmapi.SecretReference{
			SecretId:   s.SecretID,
			SecretName: s.SecretName,
		}
		if s.File != nil {
			ref.Target = &swarmapi.SecretReference_File{
				File: &swarmapi.FileTarget{
					Name: s.File.Name,
					Uid:  s.File.UID,
					Gid:  s.File.GID,
					Mode: uint32(s.File.Mode),
				},
			}
		}

		refs = append(refs, ref)
	}

	return refs
}

func secretReferencesFromGRPC(sr []*swarmapi.SecretReference) []*types.SecretReference {
	refs := make([]*types.SecretReference, 0, len(sr))
	for _, s := range sr {
		target := s.GetFile()
		if target == nil {
			// not a file target
			log.G(context.TODO()).Warnf("secret target not a file: secret=%s", s.SecretId)
			continue
		}
		refs = append(refs, &types.SecretReference{
			File: &types.SecretReferenceFileTarget{
				Name: target.Name,
				UID:  target.Uid,
				GID:  target.Gid,
				Mode: os.FileMode(target.Mode),
			},
			SecretID:   s.SecretId,
			SecretName: s.SecretName,
		})
	}

	return refs
}

func configReferencesToGRPC(sr []*types.ConfigReference) ([]*swarmapi.ConfigReference, error) {
	refs := make([]*swarmapi.ConfigReference, 0, len(sr))
	for _, s := range sr {
		ref := &swarmapi.ConfigReference{
			ConfigId:   s.ConfigID,
			ConfigName: s.ConfigName,
		}
		switch {
		case s.Runtime == nil && s.File == nil:
			return nil, errors.New("either File or Runtime should be set")
		case s.Runtime != nil && s.File != nil:
			return nil, errors.New("cannot specify both File and Runtime")
		case s.Runtime != nil:
			// Runtime target was added in API v1.40 and takes precedence over
			// File target. However, File and Runtime targets are mutually exclusive,
			// so we should never have both.
			ref.Target = &swarmapi.ConfigReference_Runtime{
				Runtime: &swarmapi.RuntimeTarget{},
			}
		case s.File != nil:
			ref.Target = &swarmapi.ConfigReference_File{
				File: &swarmapi.FileTarget{
					Name: s.File.Name,
					Uid:  s.File.UID,
					Gid:  s.File.GID,
					Mode: uint32(s.File.Mode),
				},
			}
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

func configReferencesFromGRPC(sr []*swarmapi.ConfigReference) []*types.ConfigReference {
	refs := make([]*types.ConfigReference, 0, len(sr))
	for _, s := range sr {
		r := &types.ConfigReference{
			ConfigID:   s.ConfigId,
			ConfigName: s.ConfigName,
		}
		if target := s.GetRuntime(); target != nil {
			r.Runtime = &types.ConfigReferenceRuntimeTarget{}
		} else if target := s.GetFile(); target != nil {
			r.File = &types.ConfigReferenceFileTarget{
				Name: target.Name,
				UID:  target.Uid,
				GID:  target.Gid,
				Mode: os.FileMode(target.Mode),
			}
		} else {
			// not a file target
			log.G(context.TODO()).Warnf("config target not known: config=%s", s.ConfigId)
			continue
		}
		refs = append(refs, r)
	}

	return refs
}

func containerToGRPC(c *types.ContainerSpec) (*swarmapi.ContainerSpec, error) {
	containerSpec := &swarmapi.ContainerSpec{
		Image:          c.Image,
		Labels:         c.Labels,
		Command:        c.Command,
		Args:           c.Args,
		Hostname:       c.Hostname,
		Env:            c.Env,
		Dir:            c.Dir,
		User:           c.User,
		Groups:         c.Groups,
		StopSignal:     c.StopSignal,
		Tty:            c.TTY,
		OpenStdin:      c.OpenStdin,
		ReadOnly:       c.ReadOnly,
		Hosts:          c.Hosts,
		Secrets:        secretReferencesToGRPC(c.Secrets),
		Isolation:      isolationToGRPC(c.Isolation),
		Init:           initToGRPC(c.Init),
		Sysctls:        c.Sysctls,
		CapabilityAdd:  c.CapabilityAdd,
		CapabilityDrop: c.CapabilityDrop,
		Ulimits:        ulimitsToGRPC(c.Ulimits),
		OomScoreAdj:    c.OomScoreAdj,
	}

	if c.DNSConfig != nil {
		containerSpec.DnsConfig = &swarmapi.ContainerSpec_DNSConfig{
			Nameservers: sliceutil.Map(c.DNSConfig.Nameservers, (netip.Addr).String),
			Search:      c.DNSConfig.Search,
			Options:     c.DNSConfig.Options,
		}
	}

	if c.StopGracePeriod != nil {
		containerSpec.StopGracePeriod = durationpb.New(*c.StopGracePeriod)
	}

	// Privileges
	if c.Privileges != nil {
		containerSpec.Privileges = &swarmapi.Privileges{}

		if c.Privileges.CredentialSpec != nil {
			cs, err := credentialSpecToGRPC(c.Privileges.CredentialSpec)
			if err != nil {
				return nil, errors.Wrap(err, "invalid CredentialSpec")
			}
			containerSpec.Privileges.CredentialSpec = cs
		}

		if c.Privileges.SELinuxContext != nil {
			containerSpec.Privileges.SelinuxContext = &swarmapi.Privileges_SELinuxContext{
				Disable: c.Privileges.SELinuxContext.Disable,
				User:    c.Privileges.SELinuxContext.User,
				Type:    c.Privileges.SELinuxContext.Type,
				Role:    c.Privileges.SELinuxContext.Role,
				Level:   c.Privileges.SELinuxContext.Level,
			}
		}

		if c.Privileges.Seccomp != nil {
			containerSpec.Privileges.Seccomp = &swarmapi.Privileges_SeccompOpts{
				Profile: c.Privileges.Seccomp.Profile,
			}

			switch c.Privileges.Seccomp.Mode {
			case types.SeccompModeDefault:
				containerSpec.Privileges.Seccomp.Mode = swarmapi.Privileges_SeccompOpts_DEFAULT
			case types.SeccompModeUnconfined:
				containerSpec.Privileges.Seccomp.Mode = swarmapi.Privileges_SeccompOpts_UNCONFINED
			case types.SeccompModeCustom:
				containerSpec.Privileges.Seccomp.Mode = swarmapi.Privileges_SeccompOpts_CUSTOM
			}
		}

		if c.Privileges.AppArmor != nil {
			containerSpec.Privileges.Apparmor = &swarmapi.Privileges_AppArmorOpts{}

			switch c.Privileges.AppArmor.Mode {
			case types.AppArmorModeDefault:
				containerSpec.Privileges.Apparmor.Mode = swarmapi.Privileges_AppArmorOpts_DEFAULT
			case types.AppArmorModeDisabled:
				containerSpec.Privileges.Apparmor.Mode = swarmapi.Privileges_AppArmorOpts_DISABLED
			}
		}

		containerSpec.Privileges.NoNewPrivileges = c.Privileges.NoNewPrivileges
	}

	if c.Configs != nil {
		configs, err := configReferencesToGRPC(c.Configs)
		if err != nil {
			return nil, errors.Wrap(err, "invalid Config")
		}
		containerSpec.Configs = configs
	}

	// Mounts
	for _, m := range c.Mounts {
		mount := swarmapi.Mount{
			Target:   m.Target,
			Source:   m.Source,
			Readonly: m.ReadOnly,
		}

		if mountType, ok := swarmapi.Mount_Type_value[strings.ToUpper(string(m.Type))]; ok {
			mount.Type = swarmapi.Mount_Type(mountType)
		} else if string(m.Type) != "" {
			return nil, fmt.Errorf("invalid MountType: %q", m.Type)
		}

		if m.BindOptions != nil {
			if mountPropagation, ok := swarmapi.Mount_BindOptions_Propagation_value[strings.ToUpper(string(m.BindOptions.Propagation))]; ok {
				mount.BindOptions = &swarmapi.Mount_BindOptions{Propagation: swarmapi.Mount_BindOptions_Propagation(mountPropagation)}
			} else if string(m.BindOptions.Propagation) != "" {
				return nil, fmt.Errorf("invalid MountPropagation: %q", m.BindOptions.Propagation)
			}

			if m.BindOptions.NonRecursive {
				if mount.BindOptions == nil {
					// the propagation defaults to rprivate
					mount.BindOptions = &swarmapi.Mount_BindOptions{}
				}
				mount.BindOptions.Nonrecursive = m.BindOptions.NonRecursive
			}
		}

		if m.VolumeOptions != nil {
			mount.VolumeOptions = &swarmapi.Mount_VolumeOptions{
				Nocopy:  m.VolumeOptions.NoCopy,
				Labels:  m.VolumeOptions.Labels,
				Subpath: m.VolumeOptions.Subpath,
			}
			if m.VolumeOptions.DriverConfig != nil {
				mount.VolumeOptions.DriverConfig = &swarmapi.Driver{
					Name:    m.VolumeOptions.DriverConfig.Name,
					Options: m.VolumeOptions.DriverConfig.Options,
				}
			}
		}

		if m.TmpfsOptions != nil {
			mount.TmpfsOptions = &swarmapi.Mount_TmpfsOptions{
				SizeBytes: m.TmpfsOptions.SizeBytes,
				Mode:      uint32(m.TmpfsOptions.Mode),
				Options:   tmpfsOptionsToGRPC(m.TmpfsOptions.Options),
			}
		}

		containerSpec.Mounts = append(containerSpec.Mounts, &mount)
	}

	if c.Healthcheck != nil {
		containerSpec.Healthcheck = healthConfigToGRPC(c.Healthcheck)
	}

	return containerSpec, nil
}

func credentialSpecFromGRPC(c *swarmapi.Privileges_CredentialSpec) *types.CredentialSpec {
	cs := &types.CredentialSpec{}
	switch c.Source.(type) {
	case *swarmapi.Privileges_CredentialSpec_Config:
		cs.Config = c.GetConfig()
	case *swarmapi.Privileges_CredentialSpec_File:
		cs.File = c.GetFile()
	case *swarmapi.Privileges_CredentialSpec_Registry:
		cs.Registry = c.GetRegistry()
	}
	return cs
}

func credentialSpecToGRPC(c *types.CredentialSpec) (*swarmapi.Privileges_CredentialSpec, error) {
	var opts []string

	if c.Config != "" {
		opts = append(opts, `"config"`)
	}
	if c.File != "" {
		opts = append(opts, `"file"`)
	}
	if c.Registry != "" {
		opts = append(opts, `"registry"`)
	}
	l := len(opts)
	switch {
	case l == 0:
		return nil, errors.New(`must either provide "file", "registry", or "config" for credential spec`)
	case l == 2:
		return nil, fmt.Errorf("cannot specify both %s and %s credential specs", opts[0], opts[1])
	case l > 2:
		return nil, fmt.Errorf("cannot specify both %s, and %s credential specs", strings.Join(opts[:l-1], ", "), opts[l-1])
	}

	spec := &swarmapi.Privileges_CredentialSpec{}
	switch {
	case c.Config != "":
		spec.Source = &swarmapi.Privileges_CredentialSpec_Config{
			Config: c.Config,
		}
	case c.File != "":
		spec.Source = &swarmapi.Privileges_CredentialSpec_File{
			File: c.File,
		}
	case c.Registry != "":
		spec.Source = &swarmapi.Privileges_CredentialSpec_Registry{
			Registry: c.Registry,
		}
	}

	return spec, nil
}

func healthConfigFromGRPC(h *swarmapi.HealthConfig) *container.HealthConfig {
	interval := h.Interval.AsDuration()
	timeout := h.Timeout.AsDuration()
	startPeriod := h.StartPeriod.AsDuration()
	startInterval := h.StartInterval.AsDuration()
	return &container.HealthConfig{
		Test:          h.Test,
		Interval:      interval,
		Timeout:       timeout,
		Retries:       int(h.Retries),
		StartPeriod:   startPeriod,
		StartInterval: startInterval,
	}
}

func healthConfigToGRPC(h *container.HealthConfig) *swarmapi.HealthConfig {
	return &swarmapi.HealthConfig{
		Test:          h.Test,
		Interval:      durationpb.New(h.Interval),
		Timeout:       durationpb.New(h.Timeout),
		Retries:       int32(h.Retries),
		StartPeriod:   durationpb.New(h.StartPeriod),
		StartInterval: durationpb.New(h.StartInterval),
	}
}

// IsolationFromGRPC converts a swarm api container isolation to a moby isolation representation
func IsolationFromGRPC(i swarmapi.ContainerSpec_Isolation) container.Isolation {
	switch i {
	case swarmapi.ContainerSpec_ISOLATION_HYPERV:
		return container.IsolationHyperV
	case swarmapi.ContainerSpec_ISOLATION_PROCESS:
		return container.IsolationProcess
	case swarmapi.ContainerSpec_ISOLATION_DEFAULT:
		return container.IsolationDefault
	}
	return container.IsolationEmpty
}

func isolationToGRPC(i container.Isolation) swarmapi.ContainerSpec_Isolation {
	if i.IsHyperV() {
		return swarmapi.ContainerSpec_ISOLATION_HYPERV
	}
	if i.IsProcess() {
		return swarmapi.ContainerSpec_ISOLATION_PROCESS
	}
	return swarmapi.ContainerSpec_ISOLATION_DEFAULT
}

func ulimitsFromGRPC(u []*swarmapi.ContainerSpec_Ulimit) []*container.Ulimit {
	ulimits := make([]*container.Ulimit, len(u))

	for i, ulimit := range u {
		ulimits[i] = &container.Ulimit{
			Name: ulimit.Name,
			Soft: ulimit.Soft,
			Hard: ulimit.Hard,
		}
	}

	return ulimits
}

func ulimitsToGRPC(u []*container.Ulimit) []*swarmapi.ContainerSpec_Ulimit {
	ulimits := make([]*swarmapi.ContainerSpec_Ulimit, len(u))

	for i, ulimit := range u {
		ulimits[i] = &swarmapi.ContainerSpec_Ulimit{
			Name: ulimit.Name,
			Soft: ulimit.Soft,
			Hard: ulimit.Hard,
		}
	}

	return ulimits
}

func tmpfsOptionsToGRPC(options [][]string) string {
	// The shape of the swarmkit API that tmpfs options are a string. The shape
	// of the docker API has them as a more structured array of arrays of
	// strings. To smooth this over, we will marshall the array-of-arrays to
	// json then pass that as the string.

	// Marshalling json can create an error, but only in specific cases which
	// are not relevant. We can ignore the possibility.
	jsonBytes, _ := json.Marshal(options) //nolint:errchkjson // ignoring errors, as described above
	return string(jsonBytes)
}

func tmpfsOptionsFromGRPC(options string) [][]string {
	// See tmpfsOptionsToGRPC for the reasoning. We undo what we did.
	var unstring [][]string
	// We can't return errors from here, so just don't ever pass anything that
	// could result in an error.
	//
	// Duh.
	//
	// If there is something erroneous, then an empty return value will result,
	// which should not be catastrophic. Because we control the data that is
	// marshalled (in tmpfsOptionsToGRPC), we can more-or-less ensure that only
	// valid data is unmarshalled here. If someone does something like muck
	// with the GRPC API directly, then they get footgun, no apologies.
	_ = json.Unmarshal([]byte(options), &unstring)
	return unstring
}
