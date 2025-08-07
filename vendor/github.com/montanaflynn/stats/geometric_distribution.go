package stats

import (
	"math"
)

// ProbGeom generates the probability for a geometric random variable
// with parameter p to achieve success in the interval of [a, b] trials
// See https://en.wikipedia.org/wiki/Geometric_distribution for more information
func ProbGeom(a int, b int, p float64) (prob float64, err error) {
	if (a > b) || (a < 1) {
		return math.NaN(), ErrBounds
	}

	prob = 0
	q := 1 - p // probability of failure

	for k := a + 1; k <= b; k++ {
		prob = prob + p*math.Pow(q, float64(k-1))
	}

	return prob, nil
}

// ProbGeom generates the expectation or average number of trials
// for a geometric random variable with parameter p
func ExpGeom(p float64) (exp float64, err error) {
	if (p > 1) || (p < 0) {
		return math.NaN(), ErrNegative
	}

	return 1 / p, nil
}

// ProbGeom generates the variance for number for a
// geometric random variable with parameter p
func VarGeom(p float64) (exp float64, err error) {
	if (p > 1) || (p < 0) {
		return math.NaN(), ErrNegative
	}
	return (1 - p) / math.Pow(p, 2), nil
}
