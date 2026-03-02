package community

/*
 * ZLint Copyright 2023 Regents of the University of Michigan
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not
 * use this file except in compliance with the License. You may obtain a copy
 * of the License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
 * implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

import (
	"crypto/rsa"
	"fmt"
	"math/big"

	"github.com/zmap/zcrypto/x509"
	"github.com/zmap/zlint/v3/lint"
	"github.com/zmap/zlint/v3/util"
)

type fermatFactorization struct {
	Rounds int `comment:"The number of iterations to attempt Fermat factorization. Note that when executing this lint against many (tens of thousands of certificates) that this configuration may have a profound affect on performance. For more information, please see https://fermatattack.secvuln.info/"`
}

func init() {
	lint.RegisterLint(&lint.Lint{
		Name: "e_rsa_fermat_factorization",
		Description: "RSA key pairs that are too close to each other are susceptible to the Fermat Factorization " +
			"Method (for more information please see https://en.wikipedia.org/wiki/Fermat%27s_factorization_method " +
			"and https://fermatattack.secvuln.info/)",
		Citation:      "Pierre de Fermat",
		Source:        lint.Community,
		EffectiveDate: util.ZeroDate,
		Lint:          NewFermatFactorization,
	})
}

func NewFermatFactorization() lint.LintInterface {
	return &fermatFactorization{Rounds: 100}
}

func (l *fermatFactorization) Configure() interface{} {
	return l
}

func (l *fermatFactorization) CheckApplies(c *x509.Certificate) bool {
	_, ok := c.PublicKey.(*rsa.PublicKey)
	return ok && c.PublicKeyAlgorithm == x509.RSA
}

func (l *fermatFactorization) Execute(c *x509.Certificate) *lint.LintResult {
	err := checkPrimeFactorsTooClose(c.PublicKey.(*rsa.PublicKey).N, l.Rounds)
	if err != nil {
		return &lint.LintResult{
			Status:  lint.Error,
			Details: fmt.Sprintf("this certificate's RSA key pair is susceptible to Fermat factorization, %s", err.Error())}
	} else {
		return &lint.LintResult{Status: lint.Pass}
	}
}

// Source: Let's Encrypt, Boulder
// Author: Aaron Gable (https://github.com/aarongable)
// Commit: https://github.com/letsencrypt/boulder/commit/89000bd61cfc6f373cb48b6f046d4fce7df5468e
//
// Returns an error if the modulus n is able to be factored into primes p and q
// via Fermat's factorization method. This method relies on the two primes being
// very close together, which means that they were almost certainly not picked
// independently from a uniform random distribution. Basically, if we can factor
// the key this easily, so can anyone else.
func checkPrimeFactorsTooClose(n *big.Int, rounds int) error {
	// Pre-allocate some big numbers that we'll use a lot down below.
	one := big.NewInt(1)
	bb := new(big.Int)

	// Any odd integer is equal to a difference of squares of integers:
	//   n = a^2 - b^2 = (a + b)(a - b)
	// Any RSA public key modulus is equal to a product of two primes:
	//   n = pq
	// Here we try to find values for a and b, since doing so also gives us the
	// prime factors p = (a + b) and q = (a - b).

	// We start with a close to the square root of the modulus n, to start with
	// two candidate prime factors that are as close together as possible and
	// work our way out from there. Specifically, we set a = ceil(sqrt(n)), the
	// first integer greater than the square root of n. Unfortunately, big.Int's
	// built-in square root function takes the floor, so we have to add one to get
	// the ceil.
	a := new(big.Int)
	a.Sqrt(n).Add(a, one)

	// We calculate b2 to see if it is a perfect square (i.e. b^2), and therefore
	// b is an integer. Specifically, b2 = a^2 - n.
	b2 := new(big.Int)
	b2.Mul(a, a).Sub(b2, n)

	for i := 0; i < rounds; i++ {
		// To see if b2 is a perfect square, we take its square root, square that,
		// and check to see if we got the same result back.
		bb.Sqrt(b2).Mul(bb, bb)
		if b2.Cmp(bb) == 0 {
			// b2 is a perfect square, so we've found integer values of a and b,
			// and can easily compute p and q as their sum and difference.
			bb.Sqrt(bb)
			p := new(big.Int).Add(a, bb)
			q := new(big.Int).Sub(a, bb)
			return fmt.Errorf("public modulus n = pq factored into p: %s; q: %s", p, q)
		}

		// Set up the next iteration by incrementing a by one and recalculating b2.
		a.Add(a, one)
		b2.Mul(a, a).Sub(b2, n)
	}
	return nil
}
