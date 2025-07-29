package stats

import "math"

// SoftMax returns the input values in the range of 0 to 1
// with sum of all the probabilities being equal to one. It
// is commonly used in machine learning neural networks.
func SoftMax(input Float64Data) ([]float64, error) {
	if input.Len() == 0 {
		return Float64Data{}, EmptyInput
	}

	s := 0.0
	c, _ := Max(input)
	for _, e := range input {
		s += math.Exp(e - c)
	}

	sm := make([]float64, len(input))
	for i, v := range input {
		sm[i] = math.Exp(v-c) / s
	}

	return sm, nil
}
