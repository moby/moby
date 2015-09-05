package dockerfile

import (
	"bufio"
	"os"
	"strings"
	"testing"
)

func TestShellParser(t *testing.T) {
	file, err := os.Open("words")
	if err != nil {
		t.Fatalf("Can't open 'words': %s", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	envs := []string{"PWD=/home", "SHELL=bash"}
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
			t.Fatalf("Error in 'words' - should be 2 words:%q", words)
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
