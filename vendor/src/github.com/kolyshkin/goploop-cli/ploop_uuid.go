package ploop

import (
	"crypto/rand"
	"fmt"
)

// UUID generates a ploop UUID
func UUID() (string, error) {
	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		return "", err
	}

	u[6] = (u[6] & 0x0F) | 0x40 // Version 4
	u[8] = (u[8] & 0x3F) | 0x80 // Variant is 10

	uuid := fmt.Sprintf("{%08x-%04x-%04x-%04x-%012x}",
		u[:4], u[4:6], u[6:8], u[8:10], u[10:])

	return uuid, nil
}
