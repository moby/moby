package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-check/check"
)

func (s *DockerSuite) TestBuildExpose(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildexpose"
	expected := "map[2375/tcp:{}]"
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 2375`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
}

func (s *DockerSuite) TestBuildExposeMorePorts(c *check.C) {
	testRequires(c, DaemonIsLinux)
	// start building docker file with a large number of ports
	portList := make([]string, 50)
	line := make([]string, 100)
	expectedPorts := make([]int, len(portList)*len(line))
	for i := 0; i < len(portList); i++ {
		for j := 0; j < len(line); j++ {
			p := i*len(line) + j + 1
			line[j] = strconv.Itoa(p)
			expectedPorts[p-1] = p
		}
		if i == len(portList)-1 {
			portList[i] = strings.Join(line, " ")
		} else {
			portList[i] = strings.Join(line, " ") + ` \`
		}
	}

	dockerfile := `FROM scratch
	EXPOSE {{range .}} {{.}}
	{{end}}`
	tmpl := template.Must(template.New("dockerfile").Parse(dockerfile))
	buf := bytes.NewBuffer(nil)
	tmpl.Execute(buf, portList)

	name := "testbuildexpose"
	_, err := buildImage(name, buf.String(), true)
	if err != nil {
		c.Fatal(err)
	}

	// check if all the ports are saved inside Config.ExposedPorts
	res, err := inspectFieldJSON(name, "Config.ExposedPorts")
	if err != nil {
		c.Fatal(err)
	}
	var exposedPorts map[string]interface{}
	if err := json.Unmarshal([]byte(res), &exposedPorts); err != nil {
		c.Fatal(err)
	}

	for _, p := range expectedPorts {
		ep := fmt.Sprintf("%d/tcp", p)
		if _, ok := exposedPorts[ep]; !ok {
			c.Errorf("Port(%s) is not exposed", ep)
		} else {
			delete(exposedPorts, ep)
		}
	}
	if len(exposedPorts) != 0 {
		c.Errorf("Unexpected extra exposed ports %v", exposedPorts)
	}
}

func (s *DockerSuite) TestBuildExposeOrder(c *check.C) {
	testRequires(c, DaemonIsLinux)
	buildID := func(name, exposed string) string {
		_, err := buildImage(name, fmt.Sprintf(`FROM scratch
		EXPOSE %s`, exposed), true)
		if err != nil {
			c.Fatal(err)
		}
		id, err := inspectField(name, "Id")
		if err != nil {
			c.Fatal(err)
		}
		return id
	}

	id1 := buildID("testbuildexpose1", "80 2375")
	id2 := buildID("testbuildexpose2", "2375 80")
	if id1 != id2 {
		c.Errorf("EXPOSE should invalidate the cache only when ports actually changed")
	}
}

func (s *DockerSuite) TestBuildExposeUpperCaseProto(c *check.C) {
	testRequires(c, DaemonIsLinux)
	name := "testbuildexposeuppercaseproto"
	expected := "map[5678/udp:{}]"
	_, err := buildImage(name,
		`FROM scratch
        EXPOSE 5678/UDP`,
		true)
	if err != nil {
		c.Fatal(err)
	}
	res, err := inspectField(name, "Config.ExposedPorts")
	if err != nil {
		c.Fatal(err)
	}
	if res != expected {
		c.Fatalf("Exposed ports %s, expected %s", res, expected)
	}
}
