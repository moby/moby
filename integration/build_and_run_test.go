package docker

import (
	"fmt"
	"github.com/dotcloud/docker"
	"strconv"
	"testing"
)

func compareContainerOutput(container *docker.Container, expected string) error {
	output, err := container.Output()
	if err == nil {
		if string(output) != expected {
			err = fmt.Errorf("'%s' != '%s'", string(output), expected)
		}
	}
	return err
}

type testContextTemplateRun struct {
	dockerfile        string
	runtimeParameters []string
	runtimeTest       func(*docker.Container, string) error
	runtimeOutput     string
	buildError        string
}

var testContextsRun = []testContextTemplateRun{
	{ //0
		`FROM {IMAGE}
RUN mkdir /data
RUN chmod 654 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{},
		compareContainerOutput,
		`40 41ac directory root root 654 /data
`,
		``,
	},
	{ //1 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ //2 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ //3
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{},
		compareContainerOutput,
		`60 41ff directory UNKNOWN UNKNOWN 777 /data
`,
		``,
	},
	{ //4 FIXME: this build error is very surprising to me
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{},
		compareContainerOutput,
		`40 41c0 directory UNKNOWN UNKNOWN 777 /data
`,
		`buildImage: The command [/bin/sh -c chmod 666 /data/ttt] returned a non-zero code: 1`,
	},
	//and now with -v /data
	{ //5 FIXME - we lose the mount point permission with -v
		`FROM {IMAGE}
RUN mkdir /data
RUN chmod 654 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 6 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 7 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 8
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/data"},
		compareContainerOutput,
		`60 41ff directory UNKNOWN UNKNOWN 777 /data
`,
		``,
	},
	{ // 9 FIXME
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/data"},
		compareContainerOutput,
		`40 41c0 directory UNKNOWN UNKNOWN 777 /data
`,
		`buildImage: The command [/bin/sh -c chmod 666 /data/ttt] returned a non-zero code: 1`,
	},
	//and now with -v /tmp:/data
	{ // 10 FIXME - lost mount point permission
		`FROM {IMAGE}
RUN mkdir /data
RUN chmod 654 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/tmp:/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 11 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/tmp:/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 12 FIXME - see https://github.com/dotcloud/docker/issues/2969
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/tmp:/data"},
		compareContainerOutput,
		`40 41c0 directory root root 700 /data
`,
		``,
	},
	{ // 13
		`FROM {IMAGE}
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
VOLUME ["/data"]
RUN chmod 755 /data
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/tmp:/data"},
		compareContainerOutput,
		`60 41ff directory UNKNOWN UNKNOWN 777 /data
`,
		``,
	},
	{ // 14 FIXME
		`FROM {IMAGE}
VOLUME ["/data"]
RUN mkdir /data ; chmod 777 /data ; chown 1000:1001 /data
RUN echo ttt > /data/ttt
RUN chmod 666 /data/ttt
CMD stat /data -c '%s %f %F %G %U %a %n'
		`,
		[]string{"-v", "/tmp:/data"},
		compareContainerOutput,
		`40 41c0 directory UNKNOWN UNKNOWN 777 /data
`,
		`buildImage: The command [/bin/sh -c chmod 666 /data/ttt] returned a non-zero code: 1`,
	},
	//TODO: add tests for --volumes-from..
}

func TestBuildAndRun(t *testing.T) {
	for idx, ctx := range testContextsRun {
		err := buildAndRun(ctx, t)
		if err != nil {
			msg := err.Error()
			if msg == ctx.buildError {
				t.Logf("Ignoring: `%s: %s`", strconv.Itoa(idx), msg)
			} else {
				t.Fatalf("'%s: %s'", strconv.Itoa(idx), msg)
			}
		}
	}
}

func buildAndRun(context testContextTemplateRun, t *testing.T) error {
	eng := NewTestEngine(t)

	img, err := buildImage(testContextTemplate{context.dockerfile,
		nil, nil}, t, eng, true)
	if err != nil {
		return err
	}

	runtime := mkRuntimeFromEngine(eng, t)
	defer nuke(runtime)

	container, _, err := mkContainer(runtime, append(context.runtimeParameters, []string{img.ID}...), t)
	if err != nil {
		return err
	}

	defer func() {
		if err := runtime.Destroy(container); err != nil {
			t.Error(err)
		}
	}()

	return context.runtimeTest(container, context.runtimeOutput)
}
