// Package cpuhotplug provieds methods to update cpuset of a restricted
// container
package cpuhotplug

import (
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

// Path to the cpuset correctly update by the kernel
const PathCpusetCpus = "/sys/fs/cgroup/cpuset/cpuset.cpus"

// ReadCurrentCpuset reads the current cpuset.
func ReadCurrentCpuset() (string, error) {
	b, err := ioutil.ReadFile(PathCpusetCpus)
	return strings.TrimSpace(string(b)), err
}

// getMaxCpuNumber retrieves the maximum number of possible cpus in the
// system.
func getMaxCpuNumber() int {

	b, err := ioutil.ReadFile("/sys/devices/system/cpu/possible")
	if err != nil {
		logrus.Fatalf("%s", err)
	}

	split := strings.Split(strings.TrimSpace(string(b)), "-")

	maxCpu, err := strconv.Atoi(split[1])
	if err != nil {
		return 1024
	}
	//It starts from 0
	return maxCpu + 1
}

// cpusetToSlice converts the cpuset string into a slice of 0s and 1s.
// A 0 indicates that the i-th cpus is offline, otherwise the cpu is online
// represented by a 1.
// The cpuset is represented by a string. A "-" stands for a continuous interval,
// of online cpus, a "," for a discontinuous.
// Example:
//	max number of cpus:12
//	cpuset 1-3,5-8n => [0 1 1 1 0 1 1 1 1 0 0 0]
func cpusetToSlice(cpuset string) []int {
	cpu := make([]int, getMaxCpuNumber())
	for _, s := range strings.Split(cpuset, ",") {
		arr := strings.Split(s, "-")
		if len(arr) == 1 {
			// single cpu
			i, _ := strconv.Atoi(s)
			cpu[i] = 1
			continue
		}
		// cpu range  2-4
		a, _ := strconv.Atoi(arr[0])
		b, _ := strconv.Atoi(arr[1])
		for i := a; i <= b; i++ {
			cpu[i] = 1
		}
	}
	return cpu
}

// sliceToString converts a slice into a string for the cpuset.
// See comment for cpusetToSlice.
func sliceToString(cpu []int) string {
	// [0, 1, 1, 1, 0, 1, 0] => 1-3,5
	var res string
	for i := range cpu {
		switch {
		case cpu[i] == 0: // ignore if cpu is not set
		case i == 0 && cpu[i+1] == 0: // ^ x 0
			res += strconv.Itoa(i) + ","
		case i == 0 && cpu[i+1] == 1: // ^ x 1
			res += strconv.Itoa(i)
		case cpu[i-1] == 0 && cpu[i+1] == 0: // 0 x 0
			res += strconv.Itoa(i) + ","
		case cpu[i-1] == 0 && cpu[i+1] == 1: // 0 x 1
			res += strconv.Itoa(i)
		case cpu[i-1] == 1 && cpu[i+1] == 0: // 1 x 0
			res += "-" + strconv.Itoa(i) + ","
		case cpu[i-1] == 1 && cpu[i+1] == 1: // 1 x 1

		default:
			logrus.Debugf("Error in parsing cpu %d res: %s", i, res)
			for _, c := range cpu {
				logrus.Debugf("CPU:%d", c)
			}
		}
	}
	res = strings.TrimRight(res, ",") // remove trailing ,
	return res
}

// NewCpusetRestrictedCont merges the current cpuset according with the container
// cpuset restrictions.
func NewCpusetRestrictedCont(currentCpusset, containerCpuset string) string {
	currSlice := cpusetToSlice(currentCpusset)
	contSlice := cpusetToSlice(containerCpuset)

	// merge
	//  currSlice	    contSlice	       new
	// a)	0		0		0
	// b)	0		1		0
	// c)	1		0		0
	// d)	1		1		1
	for i := range currSlice {
		// b case
		if currSlice[i] == 0 && contSlice[i] == 1 {
			contSlice[i] = 0
		}
	}
	return sliceToString(contSlice)
}

// ListenToCpuEvent triggers the channel ch when a online
// cpu event occurs
func ListenToCpuEvent(ch chan struct{}) {

	//udevam command monitors the cpu events
	//stdbuf avoids the buffering of the output stream of udevadm command
	cmd := exec.Command("stdbuf", "-o0", "-i0", "udevadm", "monitor", "--subsystem-match=cpu", "--kernel")
	stdout, err := cmd.StdoutPipe()
	buf := make([]byte, 10000)

	if err != nil {
		logrus.Fatal(err)
	}
	// Start the monitor for cpu events
	go func() {
		if err := cmd.Start(); err != nil {
			logrus.Fatal(err)
		}
	}()

	// Filter the output to catch the cpu event and trigger ch
	go func() {
		for {
			if _, err := stdout.Read(buf); err != nil {
				break
			}
			event := string(buf)
			if strings.Contains(event, "online") {
				ch <- struct{}{}
			}
		}
	}()

}
