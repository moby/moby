package formatter

import (
	"fmt"
	"strings"
	"time"

	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/cli/command/inspect"
	units "github.com/docker/go-units"
)

const serviceInspectPrettyTemplate Format = `
ID:		{{.ID}}
Name:		{{.Name}}
{{- if .Labels }}
Labels:
{{- range $k, $v := .Labels }}
 {{ $k }}{{if $v }}={{ $v }}{{ end }}
{{- end }}{{ end }}
Service Mode:
{{- if .IsModeGlobal }}	Global
{{- else if .IsModeReplicated }}	Replicated
{{- if .ModeReplicatedReplicas }}
 Replicas:	{{ .ModeReplicatedReplicas }}
{{- end }}{{ end }}
{{- if .HasUpdateStatus }}
UpdateStatus:
 State:		{{ .UpdateStatusState }}
 Started:	{{ .UpdateStatusStarted }}
{{- if .UpdateIsCompleted }}
 Completed:	{{ .UpdateStatusCompleted }}
{{- end }}
 Message:	{{ .UpdateStatusMessage }}
{{- end }}
Placement:
{{- if .TaskPlacementConstraints -}}
 Contraints:	{{ .TaskPlacementConstraints }}
{{- end }}
{{- if .HasUpdateConfig }}
UpdateConfig:
 Parallelism:	{{ .UpdateParallelism }}
{{- if .HasUpdateDelay -}}
 Delay:		{{ .UpdateDelay }}
{{- end }}
 On failure:	{{ .UpdateOnFailure }}
{{- end }}
ContainerSpec:
 Image:		{{ .ContainerImage }}
{{- if .ContainerArgs }}
 Args:		{{ range $arg := .ContainerArgs }}{{ $arg }} {{ end }}
{{- end -}}
{{- if .ContainerEnv }}
 Env:		{{ range $env := .ContainerEnv }}{{ $env }} {{ end }}
{{- end -}}
{{- if .ContainerWorkDir }}
 Dir:		{{ .ContainerWorkDir }}
{{- end -}}
{{- if .ContainerUser }}
 User: {{ .ContainerUser }}
{{- end }}
{{- if .ContainerMounts }}
Mounts:
{{- end }}
{{- range $mount := .ContainerMounts }}
  Target = {{ $mount.Target }}
   Source = {{ $mount.Source }}
   ReadOnly = {{ $mount.ReadOnly }}
   Type = {{ $mount.Type }}
{{- end -}}
{{- if .HasResources }}
Resources:
{{- if .HasResourceReservations }}
 Reservations:
{{- if gt .ResourceReservationNanoCPUs 0.0 }}
  CPU:		{{ .ResourceReservationNanoCPUs }}
{{- end }}
{{- if .ResourceReservationMemory }}
  Memory:	{{ .ResourceReservationMemory }}
{{- end }}{{ end }}
{{- if .HasResourceLimits }}
 Limits:
{{- if gt .ResourceLimitsNanoCPUs 0.0 }}
  CPU:		{{ .ResourceLimitsNanoCPUs }}
{{- end }}
{{- if .ResourceLimitMemory }}
  Memory:	{{ .ResourceLimitMemory }}
{{- end }}{{ end }}{{ end }}
{{- if .Networks }}
Networks:
{{- range $network := .Networks }} {{ $network }}{{ end }} {{ end }}
Endpoint Mode:	{{ .EndpointMode }}
{{- if .Ports }}
Ports:
{{- range $port := .Ports }}
 PublishedPort {{ $port.PublishedPort }}
  Protocol = {{ $port.Protocol }}
  TargetPort = {{ $port.TargetPort }}
{{- end }} {{ end -}}
`

// NewServiceFormat returns a Format for rendering using a Context
func NewServiceFormat(source string) Format {
	switch source {
	case PrettyFormatKey:
		return serviceInspectPrettyTemplate
	default:
		return Format(strings.TrimPrefix(source, RawFormatKey))
	}
}

// ServiceInspectWrite renders the context for a list of services
func ServiceInspectWrite(ctx Context, refs []string, getRef inspect.GetRefFunc) error {
	if ctx.Format != serviceInspectPrettyTemplate {
		return inspect.Inspect(ctx.Output, refs, string(ctx.Format), getRef)
	}
	render := func(format func(subContext subContext) error) error {
		for _, ref := range refs {
			serviceI, _, err := getRef(ref)
			if err != nil {
				return err
			}
			service, ok := serviceI.(swarm.Service)
			if !ok {
				return fmt.Errorf("got wrong object to inspect")
			}
			if err := format(&serviceInspectContext{Service: service}); err != nil {
				return err
			}
		}
		return nil
	}
	return ctx.Write(&serviceInspectContext{}, render)
}

type serviceInspectContext struct {
	swarm.Service
	subContext
}

func (ctx *serviceInspectContext) ID() string {
	return ctx.Service.ID
}

func (ctx *serviceInspectContext) Name() string {
	return ctx.Service.Spec.Name
}

func (ctx *serviceInspectContext) Labels() map[string]string {
	return ctx.Service.Spec.Labels
}

func (ctx *serviceInspectContext) IsModeGlobal() bool {
	return ctx.Service.Spec.Mode.Global != nil
}

func (ctx *serviceInspectContext) IsModeReplicated() bool {
	return ctx.Service.Spec.Mode.Replicated != nil
}

func (ctx *serviceInspectContext) ModeReplicatedReplicas() *uint64 {
	return ctx.Service.Spec.Mode.Replicated.Replicas
}

func (ctx *serviceInspectContext) HasUpdateStatus() bool {
	return ctx.Service.UpdateStatus.State != ""
}

func (ctx *serviceInspectContext) UpdateStatusState() swarm.UpdateState {
	return ctx.Service.UpdateStatus.State
}

func (ctx *serviceInspectContext) UpdateStatusStarted() string {
	return units.HumanDuration(time.Since(ctx.Service.UpdateStatus.StartedAt))
}

func (ctx *serviceInspectContext) UpdateIsCompleted() bool {
	return ctx.Service.UpdateStatus.State == swarm.UpdateStateCompleted
}

func (ctx *serviceInspectContext) UpdateStatusCompleted() string {
	return units.HumanDuration(time.Since(ctx.Service.UpdateStatus.CompletedAt))
}

func (ctx *serviceInspectContext) UpdateStatusMessage() string {
	return ctx.Service.UpdateStatus.Message
}

func (ctx *serviceInspectContext) TaskPlacementConstraints() []string {
	if ctx.Service.Spec.TaskTemplate.Placement != nil {
		return ctx.Service.Spec.TaskTemplate.Placement.Constraints
	}
	return nil
}

func (ctx *serviceInspectContext) HasUpdateConfig() bool {
	return ctx.Service.Spec.UpdateConfig != nil
}

func (ctx *serviceInspectContext) UpdateParallelism() uint64 {
	return ctx.Service.Spec.UpdateConfig.Parallelism
}

func (ctx *serviceInspectContext) HasUpdateDelay() bool {
	return ctx.Service.Spec.UpdateConfig.Delay.Nanoseconds() > 0
}

func (ctx *serviceInspectContext) UpdateDelay() time.Duration {
	return ctx.Service.Spec.UpdateConfig.Delay
}

func (ctx *serviceInspectContext) UpdateOnFailure() string {
	return ctx.Service.Spec.UpdateConfig.FailureAction
}

func (ctx *serviceInspectContext) ContainerImage() string {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.Image
}

func (ctx *serviceInspectContext) ContainerArgs() []string {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.Args
}

func (ctx *serviceInspectContext) ContainerEnv() []string {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.Env
}

func (ctx *serviceInspectContext) ContainerWorkDir() string {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.Dir
}

func (ctx *serviceInspectContext) ContainerUser() string {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.User
}

func (ctx *serviceInspectContext) ContainerMounts() []mounttypes.Mount {
	return ctx.Service.Spec.TaskTemplate.ContainerSpec.Mounts
}

func (ctx *serviceInspectContext) HasResources() bool {
	return ctx.Service.Spec.TaskTemplate.Resources != nil
}

func (ctx *serviceInspectContext) HasResourceReservations() bool {
	return ctx.Service.Spec.TaskTemplate.Resources.Reservations.NanoCPUs > 0 || ctx.Service.Spec.TaskTemplate.Resources.Reservations.MemoryBytes > 0
}

func (ctx *serviceInspectContext) ResourceReservationNanoCPUs() float64 {
	if ctx.Service.Spec.TaskTemplate.Resources.Reservations.NanoCPUs == 0 {
		return float64(0)
	}
	return float64(ctx.Service.Spec.TaskTemplate.Resources.Reservations.NanoCPUs) / 1e9
}

func (ctx *serviceInspectContext) ResourceReservationMemory() string {
	if ctx.Service.Spec.TaskTemplate.Resources.Reservations.MemoryBytes == 0 {
		return ""
	}
	return units.BytesSize(float64(ctx.Service.Spec.TaskTemplate.Resources.Reservations.MemoryBytes))
}

func (ctx *serviceInspectContext) HasResourceLimits() bool {
	return ctx.Service.Spec.TaskTemplate.Resources.Limits.NanoCPUs > 0 || ctx.Service.Spec.TaskTemplate.Resources.Limits.MemoryBytes > 0
}

func (ctx *serviceInspectContext) ResourceLimitsNanoCPUs() float64 {
	return float64(ctx.Service.Spec.TaskTemplate.Resources.Limits.NanoCPUs) / 1e9
}

func (ctx *serviceInspectContext) ResourceLimitMemory() string {
	if ctx.Service.Spec.TaskTemplate.Resources.Limits.MemoryBytes == 0 {
		return ""
	}
	return units.BytesSize(float64(ctx.Service.Spec.TaskTemplate.Resources.Limits.MemoryBytes))
}

func (ctx *serviceInspectContext) Networks() []string {
	var out []string
	for _, n := range ctx.Service.Spec.Networks {
		out = append(out, n.Target)
	}
	return out
}

func (ctx *serviceInspectContext) EndpointMode() string {
	if ctx.Service.Spec.EndpointSpec == nil {
		return ""
	}

	return string(ctx.Service.Spec.EndpointSpec.Mode)
}

func (ctx *serviceInspectContext) Ports() []swarm.PortConfig {
	return ctx.Service.Endpoint.Ports
}
