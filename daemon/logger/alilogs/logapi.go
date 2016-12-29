// Pakcage alilogs api interface

package alilogs

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/galaxydi/go-loghub"
)

// AliLogAPI define log api interface
type AliLogAPI interface {
	PutLogs(*sls.LogGroup) error
}

// AliLogClient implements AliLogAPI interface
type AliLogClient struct {
	Endpoint        string
	ProjectName     string
	LogstoreName    string
	accessKeyID     string
	accessKeySecret string
	sessionToken    string
	project         *sls.LogProject
	logstore        *sls.LogStore
}

// PutLogs implements ali PutLogs method
func (client *AliLogClient) PutLogs(logGroup *sls.LogGroup) error {
	return client.logstore.PutLogs(logGroup)
}

// NewAliLogClient ...
func NewAliLogClient(serviceEndpoint, projectName, logstoreName, accessID, accessSecret, token string) (AliLogAPI, error) {
	client := AliLogClient{}
	client.Endpoint = serviceEndpoint
	client.ProjectName = projectName
	client.LogstoreName = logstoreName
	client.accessKeyID = accessID
	client.accessKeySecret = accessSecret
	client.sessionToken = token

	logrus.WithFields(logrus.Fields{
		"endpoint":     serviceEndpoint,
		"projectName":  projectName,
		"logstoreName": logstoreName,
	}).Info("Created alilogs client")

	logProject, err := sls.NewLogProject(projectName, serviceEndpoint, accessID, accessSecret)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("Could not get ali log project")
		return nil, errors.New("Could not get ali log project")
	}
	if client.sessionToken != "" {
		logProject.WithToken(client.sessionToken)
	}
	client.project = logProject

	logStore, err := client.project.GetLogStore(logstoreName)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"error": err,
		}).Error("Could not get ali logstore")
		return nil, errors.New("Could not get ali logstore")
	}
	client.logstore = logStore

	return &client, nil
}
