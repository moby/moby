package swarm

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
)

func generateRandomSecret() string {
	var secretBytes [generatedSecretEntropyBytes]byte

	if _, err := cryptorand.Read(secretBytes[:]); err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}

	var nn big.Int
	nn.SetBytes(secretBytes[:])
	return fmt.Sprintf("%0[1]*s", maxGeneratedSecretLength, nn.Text(generatedSecretBase))
}
