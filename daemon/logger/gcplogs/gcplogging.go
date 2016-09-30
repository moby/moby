package gcplogs

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/docker/docker/daemon/logger"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/cloud/compute/metadata"
	"google.golang.org/cloud/logging"
)

const (
	name = "gcplogs"

	projectOptKey = "gcp-project"
	logLabelsKey  = "labels"
	logEnvKey     = "env"
	logCmdKey     = "gcp-log-cmd"
	logZoneKey    = "gcp-meta-zone"
	logNameKey    = "gcp-meta-name"
	logIDKey      = "gcp-meta-id"
)

var (
	// The number of logs the gcplogs driver has dropped.
	droppedLogs uint64

	onGCE bool

	// instance metadata populated from the metadata server if available
	projectID    string
	zone         string
	instanceName string
	instanceID   string
)

func init() {

	if err := logger.RegisterLogDriver(name, New); err != nil {
		logrus.Fatal(err)
	}

	if err := logger.RegisterLogOptValidator(name, ValidateLogOpts); err != nil {
		logrus.Fatal(err)
	}
}

type gcplogs struct {
	client    *logging.Client
	instance  *instanceInfo
	container *containerInfo
}

type dockerLogEntry struct {
	Instance  *instanceInfo  `json:"instance,omitempty"`
	Container *containerInfo `json:"container,omitempty"`
	Data      string         `json:"data,omitempty"`
}

type instanceInfo struct {
	Zone string `json:"zone,omitempty"`
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

type containerInfo struct {
	Name      string            `json:"name,omitempty"`
	ID        string            `json:"id,omitempty"`
	ImageName string            `json:"imageName,omitempty"`
	ImageID   string            `json:"imageId,omitempty"`
	Created   time.Time         `json:"created,omitempty"`
	Command   string            `json:"command,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

var initGCPOnce sync.Once

func initGCP() {
	initGCPOnce.Do(func() {
		onGCE = metadata.OnGCE()
		if onGCE {
			// These will fail on instances if the metadata service is
			// down or the client is compiled with an API version that
			// has been removed. Since these are not vital, let's ignore
			// them and make their fields in the dockeLogEntry ,omitempty
			projectID, _ = metadata.ProjectID()
			zone, _ = metadata.Zone()
			instanceName, _ = metadata.InstanceName()
			instanceID, _ = metadata.InstanceID()
		}
	})
}

// New creates a new logger that logs to Google Cloud Logging using the application
// default credentials.
//
// See https://developers.google.com/identity/protocols/application-default-credentials
func New(ctx logger.Context) (logger.Logger, error) {
	initGCP()

	var project string
	if projectID != "" {
		project = projectID
	}
	if projectID, found := ctx.Config[projectOptKey]; found {
		project = projectID
	}
	if project == "" {
		return nil, fmt.Errorf("No project was specified and couldn't read project from the meatadata server. Please specify a project")
	}

	c, err := logging.NewClient(context.Background(), project, "gcplogs-docker-driver")
	if err != nil {
		return nil, err
	}

	if err := c.Ping(); err != nil {
		return nil, fmt.Errorf("unable to connect or authenticate with Google Cloud Logging: %v", err)
	}

	l := &gcplogs{
		client: c,
		container: &containerInfo{
			Name:      ctx.ContainerName,
			ID:        ctx.ContainerID,
			ImageName: ctx.ContainerImageName,
			ImageID:   ctx.ContainerImageID,
			Created:   ctx.ContainerCreated,
			Metadata:  ctx.ExtraAttributes(nil),
		},
	}

	if ctx.Config[logCmdKey] == "true" {
		l.container.Command = ctx.Command()
	}

	if onGCE {
		l.instance = &instanceInfo{
			Zone: zone,
			Name: instanceName,
			ID:   instanceID,
		}
	} else if ctx.Config[logZoneKey] != "" || ctx.Config[logNameKey] != "" || ctx.Config[logIDKey] != "" {
		l.instance = &instanceInfo{
			Zone: ctx.Config[logZoneKey],
			Name: ctx.Config[logNameKey],
			ID:   ctx.Config[logIDKey],
		}
	}

	// The logger "overflows" at a rate of 10,000 logs per second and this
	// overflow func is called. We want to surface the error to the user
	// without overly spamming /var/log/docker.log so we log the first time
	// we overflow and every 1000th time after.
	c.Overflow = func(_ *logging.Client, _ logging.Entry) error {
		if i := atomic.AddUint64(&droppedLogs, 1); i%1000 == 1 {
			logrus.Errorf("gcplogs driver has dropped %v logs", i)
		}
		return nil
	}

	return l, nil
}

// ValidateLogOpts validates the opts passed to the gcplogs driver. Currently, the gcplogs
// driver doesn't take any arguments.
func ValidateLogOpts(cfg map[string]string) error {
	for k := range cfg {
		switch k {
		case projectOptKey, logLabelsKey, logEnvKey, logCmdKey, logZoneKey, logNameKey, logIDKey:
		default:
			return fmt.Errorf("%q is not a valid option for the gcplogs driver", k)
		}
	}
	return nil
}

func (l *gcplogs) Log(m *logger.Message) error {
	return l.client.Log(logging.Entry{
		Time: m.Timestamp,
		Payload: &dockerLogEntry{
			Instance:  l.instance,
			Container: l.container,
			Data:      string(m.Line),
		},
	})
}

func (l *gcplogs) Close() error {
	return l.client.Flush()
}

func (l *gcplogs) Name() string {
	return name
}
