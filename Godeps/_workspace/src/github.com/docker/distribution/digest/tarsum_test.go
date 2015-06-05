package digest

import (
	"reflect"
	"testing"
)

func TestParseTarSumComponents(t *testing.T) {
	for _, testcase := range []struct {
		input    string
		expected TarSumInfo
		err      error
	}{
		{
			input: "tarsum.v1+sha256:220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e",
			expected: TarSumInfo{
				Version:   "v1",
				Algorithm: "sha256",
				Digest:    "220a60ecd4a3c32c282622a625a54db9ba0ff55b5ba9c29c7064a2bc358b6a3e",
			},
		},
		{
			input: "",
			err:   InvalidTarSumError(""),
		},
		{
			input: "purejunk",
			err:   InvalidTarSumError("purejunk"),
		},
		{
			input: "tarsum.v23+test:12341234123412341effefefe",
			expected: TarSumInfo{
				Version:   "v23",
				Algorithm: "test",
				Digest:    "12341234123412341effefefe",
			},
		},

		// The following test cases are ported from docker core
		{
			// Version 0 tarsum
			input: "tarsum+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			expected: TarSumInfo{
				Algorithm: "sha256",
				Digest:    "e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			},
		},
		{
			// Dev version tarsum
			input: "tarsum.dev+sha256:e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			expected: TarSumInfo{
				Version:   "dev",
				Algorithm: "sha256",
				Digest:    "e58fcf7418d4390dec8e8fb69d88c06ec07039d651fedd3aa72af9972e7d046b",
			},
		},
	} {
		tsi, err := ParseTarSum(testcase.input)
		if err != nil {
			if testcase.err != nil && err == testcase.err {
				continue // passes
			}

			t.Fatalf("unexpected error parsing tarsum: %v", err)
		}

		if testcase.err != nil {
			t.Fatalf("expected error not encountered on %q: %v", testcase.input, testcase.err)
		}

		if !reflect.DeepEqual(tsi, testcase.expected) {
			t.Fatalf("expected tarsum info: %v != %v", tsi, testcase.expected)
		}

		if testcase.input != tsi.String() {
			t.Fatalf("input should equal output: %q != %q", tsi.String(), testcase.input)
		}
	}
}
