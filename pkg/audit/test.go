// +build linux

package audit

import (
	"fmt"
)

func main() {
	if AuditValueNeedsEncoding("test") {
		fmt.Printf("Failed test 1\n")
		return
	}
	if !AuditValueNeedsEncoding("test test") {
		fmt.Printf("Failed test 2\n")
		return
	}
	fmt.Printf("Success\n")
}
