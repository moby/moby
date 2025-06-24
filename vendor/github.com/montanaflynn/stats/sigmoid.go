package stats

import "math"

// Sigmoid returns the input values in the range of -1 to 1
// along the sigmoid or s-shaped curve, commonly used in
// machine learning while training neural networks as an
// activation function.
func Sigmoid(input Float64Data) ([]float64, error) {
	if input.Len() == 0 {
		return Float64Data{}, EmptyInput
	}
	s := make([]float64, len(input))
	for i, v := range input {
		s[i] = 1 / (1 + math.Exp(-v))
	}
	return s, nil
}
