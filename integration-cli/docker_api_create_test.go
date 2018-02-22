package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration-cli/checker"
	"github.com/docker/docker/integration-cli/request"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestAPICreateWithInvalidHealthcheckParams(c *check.C) {
	// test invalid Interval in Healthcheck: less than 0s
	name := "test1"
	config := map[string]interface{}{
		"Image": "busybox",
		"Healthcheck": map[string]interface{}{
			"Interval": -10 * time.Millisecond,
			"Timeout":  time.Second,
			"Retries":  int(1000),
		},
	}

	res, body, err := request.Post("/containers/create?name="+name, request.JSONBody(config))
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)

	buf, err := request.ReadBody(body)
	c.Assert(err, checker.IsNil)

	expected := fmt.Sprintf("Interval in Healthcheck cannot be less than %s", container.MinimumDuration)
	c.Assert(getErrorMessage(c, buf), checker.Contains, expected)

	// test invalid Interval in Healthcheck: larger than 0s but less than 1ms
	name = "test2"
	config = map[string]interface{}{
		"Image": "busybox",
		"Healthcheck": map[string]interface{}{
			"Interval": 500 * time.Microsecond,
			"Timeout":  time.Second,
			"Retries":  int(1000),
		},
	}
	res, body, err = request.Post("/containers/create?name="+name, request.JSONBody(config))
	c.Assert(err, check.IsNil)

	buf, err = request.ReadBody(body)
	c.Assert(err, checker.IsNil)

	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)
	c.Assert(getErrorMessage(c, buf), checker.Contains, expected)

	// test invalid Timeout in Healthcheck: less than 1ms
	name = "test3"
	config = map[string]interface{}{
		"Image": "busybox",
		"Healthcheck": map[string]interface{}{
			"Interval": time.Second,
			"Timeout":  -100 * time.Millisecond,
			"Retries":  int(1000),
		},
	}
	res, body, err = request.Post("/containers/create?name="+name, request.JSONBody(config))
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)

	buf, err = request.ReadBody(body)
	c.Assert(err, checker.IsNil)

	expected = fmt.Sprintf("Timeout in Healthcheck cannot be less than %s", container.MinimumDuration)
	c.Assert(getErrorMessage(c, buf), checker.Contains, expected)

	// test invalid Retries in Healthcheck: less than 0
	name = "test4"
	config = map[string]interface{}{
		"Image": "busybox",
		"Healthcheck": map[string]interface{}{
			"Interval": time.Second,
			"Timeout":  time.Second,
			"Retries":  int(-10),
		},
	}
	res, body, err = request.Post("/containers/create?name="+name, request.JSONBody(config))
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)

	buf, err = request.ReadBody(body)
	c.Assert(err, checker.IsNil)

	expected = "Retries in Healthcheck cannot be negative"
	c.Assert(getErrorMessage(c, buf), checker.Contains, expected)

	// test invalid StartPeriod in Healthcheck: not 0 and less than 1ms
	name = "test3"
	config = map[string]interface{}{
		"Image": "busybox",
		"Healthcheck": map[string]interface{}{
			"Interval":    time.Second,
			"Timeout":     time.Second,
			"Retries":     int(1000),
			"StartPeriod": 100 * time.Microsecond,
		},
	}
	res, body, err = request.Post("/containers/create?name="+name, request.JSONBody(config))
	c.Assert(err, check.IsNil)
	c.Assert(res.StatusCode, check.Equals, http.StatusBadRequest)

	buf, err = request.ReadBody(body)
	c.Assert(err, checker.IsNil)

	expected = fmt.Sprintf("StartPeriod in Healthcheck cannot be less than %s", container.MinimumDuration)
	c.Assert(getErrorMessage(c, buf), checker.Contains, expected)
}
