package parallel

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/docker/docker/pkg/dag"
	"github.com/docker/docker/pkg/testutil/assert"
)

func parseDockerfile(t *testing.T, b []byte) *parser.Node {
	directive := parser.Directive{
		EscapeSeen:           false,
		LookingForDirectives: true,
	}
	parser.SetEscapeToken(parser.DefaultEscapeToken, &directive)
	rootNode, err := parser.Parse(ioutil.NopCloser(bytes.NewReader(b)), &directive)
	if err != nil {
		t.Fatal(err)
	}
	return rootNode
}

func testParseStages(t *testing.T, df *parser.Node) ([]*Stage, *dag.Graph) {
	t.Logf("=== Input ===")
	t.Logf("dockerfile dump: %q", df.Dump())
	stages, err := ParseStages(df)
	assert.NilError(t, err)
	t.Logf("parsed %d stages", len(stages))
	for i, st := range stages {
		t.Logf("")
		t.Logf("=== Stage %d ===", i)
		t.Logf("name: %q", st.Name)
		t.Logf("dependency: %v", st.Dependency)
		t.Logf("dependency stage indice: %v", ComputeAllDependencyStages(stages, i))
		t.Logf("dockerfile dump: %q", st.Dockerfile.Dump())
	}
	graph, err := CreateDAG(stages)
	assert.NilError(t, err)
	t.Logf("graph: %#v", graph)
	return stages, graph
}

// DAG should have 4 nodes, 2 edges (2->0, 3->1)
var testDockerfile1 = []byte(`FROM busybox AS foo
RUN echo dummy-foo \
apple \
pineapple > /dummy-foo-out
FROM busybox AS bar
RUN echo dummy-bar > /dummy-bar-out
FROM busybox
COPY --from foo /dummy-foo-out \
/x
RUN cat /x
FROM busybox
COPY --from bar /dummy-bar-out /x
COPY --from docker.io/library/nginx /etc/passwd /y
RUN cat /x /y`)

func TestParseStages1(t *testing.T) {
	df := parseDockerfile(t, testDockerfile1)
	stages, graph := testParseStages(t, df)
	assert.Equal(t, len(stages), 4)
	assert.Equal(t, stages[0].Name, "foo")
	assert.Equal(t, len(stages[0].Dependency), 0)
	assert.Equal(t, stages[1].Name, "bar")
	assert.Equal(t, len(stages[1].Dependency), 0)
	assert.Equal(t, stages[2].Name, "")
	assert.DeepEqual(t, stages[2].Dependency, []string{"foo"})
	assert.Equal(t, stages[3].Name, "")
	assert.DeepEqual(t, stages[3].Dependency, []string{"bar", "docker.io/library/nginx"})
	assert.DeepEqual(t, graph,
		&dag.Graph{
			Nodes: []dag.Node{0, 1, 2, 3},
			Edges: []dag.Edge{
				{Depender: 2, Dependee: 0},
				{Depender: 3, Dependee: 1},
			},
		})
}

var testDockerfile2 = []byte(`FROM busybox AS foo
RUN echo foo begin
RUN echo foo end

FROM busybox AS bar
RUN echo bar begin
RUN sleep 5
RUN echo bar end

FROM busybox AS baz
RUN echo baz begin
COPY --from=foo /bin/busybox /tmp/dummy
RUN echo baz end`)

func TestParseStages2(t *testing.T) {
	df := parseDockerfile(t, testDockerfile2)
	stages, graph := testParseStages(t, df)
	assert.Equal(t, len(stages), 3)
	assert.Equal(t, stages[0].Name, "foo")
	assert.Equal(t, len(stages[0].Dependency), 0)
	assert.Equal(t, stages[1].Name, "bar")
	assert.Equal(t, len(stages[1].Dependency), 0)
	assert.Equal(t, stages[2].Name, "baz")
	assert.DeepEqual(t, stages[2].Dependency, []string{"foo"})
	assert.DeepEqual(t, graph,
		&dag.Graph{
			Nodes: []dag.Node{0, 1, 2},
			Edges: []dag.Edge{
				{Depender: 2, Dependee: 0},
			},
		})

	injected := InjectDependencyStageImageIDsToDockerfile(stages[2].Dockerfile,
		map[string]string{
			"foo": "sha256:deadbeef",
		})
	t.Logf("=== inject ===")
	t.Log(injected.Dump())
		
}
