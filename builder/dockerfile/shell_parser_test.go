package dockerfile

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestShellParser4EnvVars(t *testing.T) {
	fn := "envVarTest"

	file, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Can't open '%s': %s", err, fn)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	envs := []string{"PWD=/home", "SHELL=bash", "KOREAN=한국어"}
	for scanner.Scan() {
		line := scanner.Text()

		// Trim comments and blank lines
		i := strings.Index(line, "#")
		if i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		words := strings.Split(line, "|")
		if len(words) != 2 {
			t.Fatalf("Error in '%s' - should be exactly one | in:%q", fn, line)
		}

		words[0] = strings.TrimSpace(words[0])
		words[1] = strings.TrimSpace(words[1])

		newWord, err := ProcessWord(words[0], envs)

		if err != nil {
			newWord = "error"
		}

		if newWord != words[1] {
			t.Fatalf("Error. Src: %s  Calc: %s  Expected: %s", words[0], newWord, words[1])
		}
	}
}

func TestShellParser4Words(t *testing.T) {
	fn := "wordsTest"

	file, err := os.Open(fn)
	if err != nil {
		t.Fatalf("Can't open '%s': %s", err, fn)
	}
	defer file.Close()

	envs := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "ENV ") {
			line = strings.TrimLeft(line[3:], " ")
			envs = append(envs, line)
			continue
		}

		words := strings.Split(line, "|")
		if len(words) != 2 {
			t.Fatalf("Error in '%s' - should be exactly one | in: %q", fn, line)
		}
		test := strings.TrimSpace(words[0])
		expected := strings.Split(strings.TrimLeft(words[1], " "), ",")

		result, err := ProcessWords(test, envs)

		if err != nil {
			result = []string{"error"}
		}

		if len(result) != len(expected) {
			t.Fatalf("Error. %q was suppose to result in %q, but got %q instead", test, expected, result)
		}
		for i, w := range expected {
			if w != result[i] {
				t.Fatalf("Error. %q was suppose to result in %q, but got %q instead", test, expected, result)
			}
		}
	}
}

func TestGetEnv(t *testing.T) {
	sw := &shellWord{
		word: "",
		envs: nil,
		pos:  0,
	}

	sw.envs = []string{}
	if sw.getEnv("foo") != "" {
		t.Fatalf("2 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo"}
	if sw.getEnv("foo") != "" {
		t.Fatalf("3 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo="}
	if sw.getEnv("foo") != "" {
		t.Fatalf("4 - 'foo' should map to ''")
	}

	sw.envs = []string{"foo=bar"}
	if sw.getEnv("foo") != "bar" {
		t.Fatalf("5 - 'foo' should map to 'bar'")
	}

	sw.envs = []string{"foo=bar", "car=hat"}
	if sw.getEnv("foo") != "bar" {
		t.Fatalf("6 - 'foo' should map to 'bar'")
	}
	if sw.getEnv("car") != "hat" {
		t.Fatalf("7 - 'car' should map to 'hat'")
	}

	// Make sure we grab the first 'car' in the list
	sw.envs = []string{"foo=bar", "car=hat", "car=bike"}
	if sw.getEnv("car") != "hat" {
		t.Fatalf("8 - 'car' should map to 'hat'")
	}
}
