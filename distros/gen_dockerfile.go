package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

// DfileConfig holds info for distribution-specific Dockerfile
type DfileConfig struct {
	Distribution string   `json:"distribution"`
	Dependencies []string `json:"dependencies"`
	Buildtags    string   `json:"buildtags"`
	Markers      []string `json:"markers"`
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
func genOS() (string, error) {
	file, err := ioutil.ReadFile("/etc/redhat-release")
	if err != nil {
		return "Dockerfile", nil
	}
	line := strings.Split(string(file), " ")
	osStr := line[0]
	switch osStr {
	case "Fedora":
		return "Fedora", nil
	case "CentOS":
		return "Centos", nil
	case "Red":
		return "RHEL", nil
	default:
		return "", nil
	}
}

func genDockerfileName(osName string) (string, error) {
	return "distros/Dockerfile" + osName, nil
}

func patchLines(patched string, original string, osName string) error {
	patchedFile, err := os.Create(patched)
	check(err)
	w := bufio.NewWriter(patchedFile)
	defer w.Flush()
	dfConfig := new(DfileConfig)
	patchJSON := fmt.Sprintf("distros/%s.json", osName)
	patchJSONFile, err := os.Open(patchJSON)
	check(err)
	defer patchJSONFile.Close()
	jsonParser := json.NewDecoder(patchJSONFile)
	check(err)
	err = jsonParser.Decode(&dfConfig)
	check(err)
	origDf, err := os.Open(original)
	check(err)
	defer origDf.Close()
	scanner := bufio.NewScanner(origDf)
	scanner.Split(bufio.ScanLines)
	i := 0 // will increment to avoid multiple writes of same lines
	for scanner.Scan() {
		switch i {
		case 0:
			if !strings.Contains(scanner.Text(), dfConfig.Markers[0]) {
				fmt.Fprintln(w, scanner.Text())
			} else {
				fmt.Fprintln(w, scanner.Text())
				fmt.Fprintln(w, dfConfig.Distribution)
				for _, dep := range dfConfig.Dependencies {
					fmt.Fprintln(w, dep)
				}
				i++
			}
		case 1:
			if strings.Contains(scanner.Text(), dfConfig.Markers[1]) {
				fmt.Fprintln(w, scanner.Text())
				i++
			}
		case 2:
			if !strings.Contains(scanner.Text(), dfConfig.Markers[2]) {
				fmt.Fprintln(w, scanner.Text())
			} else {
				fmt.Fprintln(w, scanner.Text())
				fmt.Fprintln(w, dfConfig.Buildtags)
				i++
			}
		case 3:
			if strings.Contains(scanner.Text(), dfConfig.Markers[3]) {
				fmt.Fprintln(w, scanner.Text())
				i++
			}
		default:
			fmt.Fprintln(w, scanner.Text())
		}
	}
	return nil
}

func main() {
	osName, err := genOS()
	if err != nil {
		log.Fatal(err)
	}
	patchedDockerfile, err := genDockerfileName(osName)
	if err != nil {
		log.Fatal(err)
	}
	if patchedDockerfile != "Dockerfile" {
		err = patchLines(patchedDockerfile, "Dockerfile", osName)
		if err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(patchedDockerfile)
}
