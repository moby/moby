package omslogs

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestValidateLogOpt(t *testing.T) {
	if err := ValidateLogOpt(map[string]string{
		"omslogs-workspaceID": "56fcb3e3-8309-4989-befd-17ddf7462181",
		"omslogs-sharedKey":   "LjdZpLJL1bhDFB4/coTPumo6rmc5CImAyfxjMklDDNc=",
	}); err != nil {
		t.Fatal(err)
	}

	if err := ValidateLogOpt(map[string]string{
		"unknown": "unknown",
	}); err == nil {
		t.Fatal("expected error for unsupported option")
	}
}

func TestValidateLogOptTimeoutInvalid(t *testing.T) {
	if err := ValidateLogOpt(map[string]string{
		"omslogs-timeout": "invalid",
	}); err == nil {
		t.Fatalf("expected error for invalid value for option '%s'", optTimeout)
	}
}

func TestValidateLogOptTimeoutNegative(t *testing.T) {
	if err := ValidateLogOpt(map[string]string{
		"omslogs-timeout": "-1ms",
	}); err == nil {
		t.Fatalf("expected error for negative option '%s'", optTimeout)
	}
}

func TestValidateLogOptTimeout(t *testing.T) {
	if err := ValidateLogOpt(map[string]string{
		"omslogs-timeout": "5s",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestNewMissedConfig(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{},
	}

	if _, err := New(info); err == nil {
		t.Fatal("expected failure when missing required options")
	}
}

func TestNewMissedWorkspaceID(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"omslogs-sharedKey": "LjdZpLJL1bhDFB4/coTPumo6rmc5CImAyfxjMklDDNc=",
		},
	}

	if _, err := New(info); err == nil {
		t.Fatalf("expected failure when missing required '%s'", optWorkspaceID)
	}
}

func TestNewMissedSharedKey(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"omslogs-workspaceID": "56fcb3e3-8309-4989-befd-17ddf7462181",
		},
	}

	if _, err := New(info); err == nil {
		t.Fatalf("expected failure when missing required '%s'", optSharedKey)
	}
}

func TestLog(t *testing.T) {
	info := logger.Info{
		Config: map[string]string{
			"omslogs-workspaceID": "56fcb3e3-8309-4989-befd-17ddf7462181",
			"omslogs-sharedKey":   "LjdZpLJL1bhDFB4/coTPumo6rmc5CImAyfxjMklDDNc=",
		},
		ContainerID:        "containerID",
		ContainerName:      "containerName",
		ContainerImageID:   "imageID",
		ContainerImageName: "imageName",
	}

	l, err := New(info)
	if err != nil {
		t.Error(err)
	}

	mock := &OmsLogClientMock{}

	sut := l.(*omsLogger)
	sut.setClient(mock)

	for i := 0; i < 2*sut.postMessagesBatchSize; i++ {
		message := &logger.Message{
			Source: "stdout",
			Line:   bytes.NewBufferString(fmt.Sprintf("message %d\n", i)).Bytes(),
		}

		if err := sut.Log(message); err != nil {
			t.Error(err)
		}
	}

	l.Close()

	messages := mock.getMessages()
	if len(messages) != 2 {
		t.Fatal("expected 2 batches of log messages")
	}
}
