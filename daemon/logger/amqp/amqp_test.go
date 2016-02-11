package amqp

import (
	"io/ioutil"
	"syscall"
	"testing"

	"github.com/docker/docker/daemon/logger"
)

func TestParseURL(t *testing.T) {
	config := map[string]string{
		"amqp-cert":     "/path/to/cert.pem",
		"amqp-key":      "/path/to/key.pem",
		"amqp-url":      "amqp://guest:guest@localhost:5672/ amqp://guest:guest@127.0.0.1:5672/",
		"amqp-confirm":  "true",
		"amqp-exchange": "log-exchange",
		"amqp-queue":    "log-queue",
	}
	l := logger.Context{
		Config: config,
	}
	brokerArray := parseURL(l)
	expectedURL1 := "amqp://guest:guest@localhost:5672/"
	if brokerArray[0].BrokerURL != expectedURL1 {
		t.Errorf("URL '%v' not parsed correctly", expectedURL1)
	}
	expectedURL2 := "amqp://guest:guest@127.0.0.1:5672/"
	if brokerArray[1].BrokerURL != expectedURL2 {
		t.Errorf("URL '%v' not parsed correctly", expectedURL2)
	}
	expectedExchange1 := "log-exchange"
	if brokerArray[0].Exchange != expectedExchange1 {
		t.Errorf("Exchange '%v' not parsed", expectedExchange1)
	}
	expectedExchange2 := "log-exchange"
	if brokerArray[1].Exchange != expectedExchange2 {
		t.Errorf("Exchange '%v' not parsed", expectedExchange2)
	}
}

func TestParseJSONFile(t *testing.T) {
	f, err := ioutil.TempFile("", "test.json")
	if err != nil {
		t.Errorf("Could not create test JSON file: %v", err)
	}
	defer syscall.Unlink(f.Name())
	ioutil.WriteFile(f.Name(), []byte("[{\"url\":\"amqp://guest:guest@localhost:5672/\", \"exchange\":\"log\", \"queue\":\"log-queue\", \"routingkey\":\"log-key\"},{\"url\":\"amqps://guest:guest@127.0.0.1:5672/\", \"exchange\":\"log\", \"queue\":\"log-queue\", \"routingkey\":\"log-key\"}]"), 0644)
	brokerArray := parseJSONFile(f.Name())
	expectedURL1 := "amqp://guest:guest@localhost:5672/"
	if brokerArray[0].BrokerURL != expectedURL1 {
		t.Errorf("URL '%v' not parsed correctly", expectedURL1)
	}
	expectedURL2 := "amqps://guest:guest@127.0.0.1:5672/"
	if brokerArray[1].BrokerURL != expectedURL2 {
		t.Errorf("URL '%v' not parsed correctly", expectedURL2)
	}
	expectedExchange1 := "log"
	if brokerArray[0].Exchange != expectedExchange1 {
		t.Errorf("Exchange '%v' not parsed", expectedExchange1)
	}
	expectedExchange2 := "log"
	if brokerArray[1].Exchange != expectedExchange2 {
		t.Errorf("Exchange '%v' not parsed", expectedExchange2)
	}
}
