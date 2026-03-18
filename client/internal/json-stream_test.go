package internal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moby/moby/api/types"
	"gotest.tools/v3/assert"
)

func Test_JsonSeqDecoder(t *testing.T) {
	separator := string(rune(rs))
	lf := "\n"
	input := fmt.Sprintf(`%s{"hello":"world"}%s%s{ "hello": "again" }%s`, separator, lf, separator, lf)
	decoder := NewJSONStreamDecoder(strings.NewReader(input), types.MediaTypeJSONSequence)
	type Hello struct {
		Hello string `json:"hello"`
	}
	var hello Hello
	err := decoder(&hello)
	assert.NilError(t, err)
	assert.Equal(t, "world", hello.Hello)

	var again Hello
	err = decoder(&again)
	assert.NilError(t, err)
	assert.Equal(t, "again", again.Hello)
}
