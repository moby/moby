package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestSingleMessageField checks that a single embedded message field (*Nested)
// generates a plain (non-repeated) proto message field and pointer-based
// conversions in both directions, mirroring how protoc-gen-go models a message
// field. The contract lives in testdata/singlemsg and is parsed, not compiled.
func TestSingleMessageField(t *testing.T) {
	pt, err := parsePoint("testdata/singlemsg")
	assert.NilError(t, err)
	pt.importPath = "example.com/singlemsg"
	pt.service = "Service"

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

// TestCamelToSnake locks down that initialisms map to clean snake_case proto
// field names (ContainerID -> container_id), not the old container_i_d.
func TestCamelToSnake(t *testing.T) {
	for in, want := range map[string]string{
		"ContainerID": "container_id",
		"Name":        "name",
		"Image":       "image",
		"HTTPServer":  "http_server",
		"APIKey":      "api_key",
		"URL":         "url",
		"AddEnv":      "add_env",
		"CapAdd":      "cap_add",
		"OCISpec":     "oci_spec",
		// A plural "s" stays attached to the acronym rather than starting a new
		// word, so these do not regress to container_i_ds / cp_us / i_ds.
		"ContainerIDs": "container_ids",
		"CPUs":         "cpus",
		"IDs":          "ids",
		"URLsAndIDs":   "urls_and_ids",
	} {
		assert.Equal(t, camelToSnake(in), want, "camelToSnake(%q)", in)
	}
}

// TestGoCamelCaseMatchesProtoc locks down that goCamelCase reproduces
// protoc-gen-go's (initialism-unaware) Go names, so the generated conversions
// address the fields and getters protoc actually emits.
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

// TestInitialismFieldBridgesNames locks down the two-name bridge: a contract
// field ContainerID emits a clean snake_case proto field (container_id) while
// the conversions still address the contract's Go name (ContainerID) and
// protoc-gen-go's Go name (ContainerId) on their respective sides.
func TestInitialismFieldBridgesNames(t *testing.T) {
	const src = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Req struct{ ContainerID string ` + "`pb:\"1\"`" + ` }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }
var Point = extensions.DefinePoint[S]("test.gen.v1")
`
	pt, err := parseSource(t, src)
	assert.NilError(t, err)
	pt.importPath = "example.com/p"
	pt.service = "Service"

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

// parseSource writes src as a contract package to a temp dir and parses it,
// so a test can drive the generator with an inline contract.
func parseSource(t *testing.T, src string) (point, error) {
	t.Helper()
	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "contract.go"), []byte(src), 0o644))
	return parsePoint(dir)
}

// TestContractValidation locks down the generator's rejection of contracts that
// would produce a broken or invalid wire format, rather than emitting it.
func TestContractValidation(t *testing.T) {
	const header = `package p
import "github.com/moby/moby/v2/internal/extensions"
type S interface{ Do(ctx interface{}, req *Req) (*Resp, error) }
type Resp struct{ Ok bool ` + "`pb:\"1\"`" + ` }
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
			name:    "zero field number",
			req:     "type Req struct{ Name string `pb:\"0\"` }",
			wantErr: "must be >= 1",
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
			// int/uint have no fixed wire width and protoc-gen-go emits
			// int64/uint64, so a direct wire assignment would not compile.
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
			// A sized integer is accepted, so the rejection is width-specific.
			name:    "sized int field",
			req:     "type Req struct{ Count int64 `pb:\"1\"` }",
			wantErr: "",
		},
		{
			// A grouped declaration would give both names field number 1.
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
