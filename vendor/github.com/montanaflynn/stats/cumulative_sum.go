package stats

// CumulativeSum calculates the cumulative sum of the input slice
func CumulativeSum(input Float64Data) ([]float64, error) {

	if input.Len() == 0 {
		return Float64Data{}, EmptyInput
	}

	cumSum := make([]float64, input.Len())

	for i, val := range input {
		if i == 0 {
			cumSum[i] = val
		} else {
			cumSum[i] = cumSum[i-1] + val
		}
	}

	return cumSum, nil
}
