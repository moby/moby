package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestSingleMessageField(t *testing.T) {
	pt, err := parsePoint("testdata/singlemsg")
	assert.NilError(t, err)
	pt.importPath = "example.com/singlemsg"

	proto, err := emitProto(pt)
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(string(proto), "Nested nested = 1;"),
		"proto should declare a single (non-repeated) message field:\n%s", proto)
	assert.Check(t, !strings.Contains(string(proto), "repeated Nested"),
		"single message field must not be repeated:\n%s", proto)

	wire, err := emitWire(pt)
	assert.NilError(t, err)
	src := string(wire)
	assert.Check(t, strings.Contains(src, "out.Nested = nestedToProto(in.Nested)"),
		"wire should convert the single message to proto by pointer:\n%s", src)
	assert.Check(t, strings.Contains(src, "out.Nested = nestedFromProto(in.GetNested())"),
		"wire should convert the single message from proto by pointer:\n%s", src)
}

func TestCamelToSnake(t *testing.T) {
	for in, want := range map[string]string{
		"ContainerID":  "container_id",
		"Name":         "name",
		"Image":        "image",
		"HTTPServer":   "http_server",
		"APIKey":       "api_key",
		"URL":          "url",
		"AddEnv":       "add_env",
		"CapAdd":       "cap_add",
		"OCISpec":      "oci_spec",
		"ContainerIDs": "container_ids",
		"CPUs":         "cpus",
		"IDs":          "ids",
		"URLsAndIDs":   "urls_and_ids",
	} {
		assert.Equal(t, camelToSnake(in), want, "camelToSnake(%q)", in)
	}
}

func TestGoCamelCaseMatchesProtoc(t *testing.T) {
	for in, want := range map[string]string{
		"container_id": "ContainerId",
		"name":         "Name",
		"http_server":  "HttpServer",
		"api_key":      "ApiKey",
		"url":          "Url",
		"add_env":      "AddEnv",
	} {
		assert.Equal(t, goCamelCase(in), want, "goCamelCase(%q)", in)
	}
}

func TestInitialismFieldBridgesNames(t *testing.T) {
	const src = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Req struct{ ContainerID string ` + "`pb:\"1\"`" + ` }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }
//mobyextgen:service=Service
var Point = extensions.DefinePoint[S]("test.gen.v1")
`
	pt, err := parseSource(t, src)
	assert.NilError(t, err)
	pt.importPath = "example.com/p"

	proto, err := emitProto(pt)
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(string(proto), "string container_id = 1;"),
		"proto field must be clean snake_case, not container_i_d:\n%s", proto)

	wire, err := emitWire(pt)
	assert.NilError(t, err)
	src2 := string(wire)
	assert.Check(t, strings.Contains(src2, "out.ContainerId = in.ContainerID"),
		"ToProto must set proto ContainerId from contract ContainerID:\n%s", src2)
	assert.Check(t, strings.Contains(src2, "out.ContainerID = in.GetContainerId()"),
		"FromProto must set contract ContainerID from proto GetContainerId():\n%s", src2)
}

func parseSource(t *testing.T, src string) (point, error) {
	t.Helper()
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "contract.go"), []byte(src), 0o644))
	return parsePoint(dir)
}

func TestContractValidation(t *testing.T) {
	const header = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }
//mobyextgen:service=Service
var Point = extensions.DefinePoint[S]("test.gen.v1")
`
	cases := []struct {
		name    string
		req     string
		wantErr string
	}{
		{
			name:    "float map key",
			req:     "type Req struct{ M map[float64]string `pb:\"1\"` }",
			wantErr: "map keys must be strings",
		},
		{
			name:    "int map key",
			req:     "type Req struct{ M map[int32]string `pb:\"1\"` }",
			wantErr: "map keys must be strings",
		},
		{
			name:    "non-numeric field number",
			req:     "type Req struct{ Name string `pb:\"one\"` }",
			wantErr: "not a field number",
		},
		{
			name:    "empty field number",
			req:     "type Req struct{ Name string `pb:\"\"` }",
			wantErr: "not a field number",
		},
		{
			name:    "zero field number",
			req:     "type Req struct{ Name string `pb:\"0\"` }",
			wantErr: "must be >= 1",
		},
		{
			name:    "oversized field number",
			req:     "type Req struct{ Name string `pb:\"536870912\"` }",
			wantErr: "must be <= 536870911",
		},
		{
			name:    "protobuf implementation-reserved field number",
			req:     "type Req struct{ Name string `pb:\"19000\"` }",
			wantErr: "reserved by the protobuf implementation",
		},
		{
			name:    "maximum field number",
			req:     "type Req struct{ Name string `pb:\"536870911\"` }",
			wantErr: "",
		},
		{
			name:    "duplicate field number",
			req:     "type Req struct{ A string `pb:\"1\"`; B string `pb:\"1\"` }",
			wantErr: "used by both",
		},
		{
			name:    "valid string map",
			req:     "type Req struct{ M map[string]string `pb:\"1\"` }",
			wantErr: "",
		},
		{
			name:    "width-ambiguous int field",
			req:     "type Req struct{ Count int `pb:\"1\"` }",
			wantErr: "no fixed width on the wire",
		},
		{
			name:    "width-ambiguous uint slice",
			req:     "type Req struct{ Counts []uint `pb:\"1\"` }",
			wantErr: "no fixed width on the wire",
		},
		{
			name:    "width-ambiguous int map value",
			req:     "type Req struct{ M map[string]int `pb:\"1\"` }",
			wantErr: "no fixed width on the wire",
		},
		{
			name:    "sized int field",
			req:     "type Req struct{ Count int64 `pb:\"1\"` }",
			wantErr: "",
		},
		{
			name:    "grouped fields share a pb tag",
			req:     "type Req struct{ A, B string `pb:\"1\"` }",
			wantErr: "share a single pb tag",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSource(t, header+tc.req+"\n")
			if tc.wantErr == "" {
				assert.NilError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestServicePragma(t *testing.T) {
	const contract = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Req struct{ Name string ` + "`pb:\"1\"`" + ` }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }
`
	t.Run("names the service", func(t *testing.T) {
		pt, err := parseSource(t, contract+"//mobyextgen:service=SomeHook\nvar Point = extensions.DefinePoint[S](\"test.gen.v1\")\n")
		assert.NilError(t, err)
		assert.Equal(t, pt.service, "SomeHook")
		assert.Equal(t, pt.grpcService(), "test.gen.v1.SomeHook")
	})

	t.Run("is required", func(t *testing.T) {
		_, err := parseSource(t, contract+"var Point = extensions.DefinePoint[S](\"test.gen.v1\")\n")
		assert.ErrorContains(t, err, "no service pragma found")
	})

	t.Run("rejects conflicting names", func(t *testing.T) {
		_, err := parseSource(t, contract+"//mobyextgen:service=A\n//mobyextgen:service=B\nvar Point = extensions.DefinePoint[S](\"test.gen.v1\")\n")
		assert.ErrorContains(t, err, "conflicting")
	})
}

func TestProtoFileNameFollowsService(t *testing.T) {
	for service, want := range map[string]string{
		"CreateSpecHook": "create_spec_hook",
		"Greeter":        "greeter",
		"Echo":           "echo",
	} {
		assert.Equal(t, camelToSnake(service), want, "proto file name for service %q", service)
	}
}

func TestServiceContractWithoutAPoint(t *testing.T) {
	const contract = `package p
type Req struct{ Name string ` + "`pb:\"1\"`" + ` }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }

//mobyextgen:service=my.proto.pkg.v1.Runtime
type Runtime interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
`
	pt, err := parseSource(t, contract)
	assert.NilError(t, err)
	assert.Equal(t, pt.iface, "Runtime")
	assert.Equal(t, pt.id, "my.proto.pkg.v1")
	assert.Equal(t, pt.service, "Runtime")
	assert.Equal(t, pt.grpcService(), "my.proto.pkg.v1.Runtime")
	assert.Check(t, !pt.isPoint, "a contract with no DefinePoint is not a point")

	pt.importPath = "example.com/p"
	wire, err := emitWire(pt)
	assert.NilError(t, err)
	src := string(wire)
	assert.Check(t, strings.Contains(src, "func RegisterServer(r grpc.ServiceRegistrar, impl p.Runtime)"), src)
	assert.Check(t, strings.Contains(src, "func NewClient(conn grpc.ClientConnInterface) p.Runtime"), src)
	assert.Check(t, !strings.Contains(src, "ServerPoint"), "a non-point contract must not emit point registrations:\n%s", src)
	assert.Check(t, !strings.Contains(src, "clientpoint"), "a non-point contract must not import the point packages:\n%s", src)
}

func TestServicePragmaOnANonInterface(t *testing.T) {
	_, err := parseSource(t, `package p
type Req struct{ Name string `+"`pb:\"1\"`"+` }
type Resp struct{ Ok bool `+"`pb:\"1\"`"+` }
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }

//mobyextgen:service=my.pkg.v1.Thing
type Thing struct{}
`)
	assert.ErrorContains(t, err, "may only document an interface or the point")
}

func TestReservedFieldNumbers(t *testing.T) {
	const header = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }

//mobyextgen:service=Service
var Point = extensions.DefinePoint[S]("test.gen.v1")
`
	t.Run("emits reserved", func(t *testing.T) {
		pt, err := parseSource(t, header+"\n//mobyextgen:reserved=2,4 were 'a' and 'b'\ntype Req struct{ Name string `pb:\"1\"` }\n")
		assert.NilError(t, err)
		pt.importPath = "example.com/p"
		proto, err := emitProto(pt)
		assert.NilError(t, err)
		assert.Check(t, strings.Contains(string(proto), "reserved 2;\n  reserved 4;"),
			"proto should reserve both burned numbers:\n%s", proto)
	})

	t.Run("rejects a field reusing a reserved number", func(t *testing.T) {
		pt, err := parseSource(t, header+"\n//mobyextgen:reserved=2\ntype Req struct{ Name string `pb:\"2\"` }\n")
		assert.NilError(t, err)
		pt.importPath = "example.com/p"
		_, err = emitProto(pt)
		assert.ErrorContains(t, err, "field number 2 is reserved")
	})

	t.Run("rejects a non-numeric reservation", func(t *testing.T) {
		_, err := parseSource(t, header+"\n//mobyextgen:reserved=two\ntype Req struct{ Name string `pb:\"1\"` }\n")
		assert.ErrorContains(t, err, "comma-separated list of field numbers")
	})

	t.Run("rejects an oversized reservation", func(t *testing.T) {
		_, err := parseSource(t, header+"\n//mobyextgen:reserved=536870912\ntype Req struct{ Name string `pb:\"1\"` }\n")
		assert.ErrorContains(t, err, "must be <= 536870911")
	})

	t.Run("descriptor accepts reservation boundaries", func(t *testing.T) {
		pt, err := parseSource(t, header+"\n//mobyextgen:reserved=19000,536870911\ntype Req struct{ Name string `pb:\"1\"` }\n")
		assert.NilError(t, err)
		fd, err := fileDescriptor(pt)
		assert.NilError(t, err)
		ranges := fd.MessageType[0].ReservedRange
		assert.Equal(t, ranges[0].GetStart(), int32(19000))
		assert.Equal(t, ranges[0].GetEnd(), int32(19001))
		assert.Equal(t, ranges[1].GetStart(), int32(536870911))
		assert.Equal(t, ranges[1].GetEnd(), int32(536870912))
	})
}

func TestSinglePointCardinality(t *testing.T) {
	const contract = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Req struct{ Name string ` + "`pb:\"1\"`" + ` }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }

//mobyextgen:service=Service
`
	t.Run("DefineSinglePoint marks the ClientPoint", func(t *testing.T) {
		pt, err := parseSource(t, contract+"var Point = extensions.DefineSinglePoint[S](\"test.gen.v1\")\n")
		assert.NilError(t, err)
		assert.Check(t, pt.isSingle)
		pt.importPath = "example.com/p"
		wire, err := emitWire(pt)
		assert.NilError(t, err)
		assert.Check(t, strings.Contains(string(wire), "Provider: ClientProvider, Single: true"),
			"the generated ClientPoint must carry the contract's cardinality:\n%s", wire)
	})

	t.Run("DefinePoint does not", func(t *testing.T) {
		pt, err := parseSource(t, contract+"var Point = extensions.DefinePoint[S](\"test.gen.v1\")\n")
		assert.NilError(t, err)
		assert.Check(t, !pt.isSingle)
		pt.importPath = "example.com/p"
		wire, err := emitWire(pt)
		assert.NilError(t, err)
		assert.Check(t, !strings.Contains(string(wire), "Single"),
			"a fan-out point must not claim single cardinality:\n%s", wire)
	})
}
