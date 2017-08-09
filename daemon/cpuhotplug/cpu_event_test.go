// +build linux
package cpuhotplug

import (
	"fmt"
	"os/exec"
	"testing"
	"time"
)

const numEvent = 15

func TestListenToCpuEvent(t *testing.T) {

	test := make(chan struct{})
	done := make(chan struct{})
	ListenToCpuEvent(test)

	// Catch the cpu event
	go func(done chan struct{}) {
		i := numEvent - 1
		for range test {
			fmt.Printf("Waiting for %d events\n", i-1)
			i--
			if i == 0 {
				done <- struct{}{}
			}
		}
	}(done)

	// Trigger the cpu events
	go func() {
		for i := 0; i < numEvent; i++ {
			//trigger cpu events
			if err := exec.Command("chcpu", "-d", "1").Run(); err != nil {
				fmt.Printf("Error %d offnline cpu\n", err)
			}
			time.Sleep(100 * time.Millisecond)
			if err := exec.Command("chcpu", "-e", " 1").Run(); err != nil {
				fmt.Printf("Error %d online cpu\n", err)
			}
			fmt.Printf("CPU event %d\n", i)
		}
	}()
	<-done
	close(test)
}
