package stats

import "math"

// MedianAbsoluteDeviation finds the median of the absolute deviations from the dataset median
func MedianAbsoluteDeviation(input Float64Data) (mad float64, err error) {
	return MedianAbsoluteDeviationPopulation(input)
}

// MedianAbsoluteDeviationPopulation finds the median of the absolute deviations from the population median
func MedianAbsoluteDeviationPopulation(input Float64Data) (mad float64, err error) {
	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}

	i := copyslice(input)
	m, _ := Median(i)

	for key, value := range i {
		i[key] = math.Abs(value - m)
	}

	return Median(i)
}

// StandardDeviation the amount of variation in the dataset
func StandardDeviation(input Float64Data) (sdev float64, err error) {
	return StandardDeviationPopulation(input)
}

// StandardDeviationPopulation finds the amount of variation from the population
func StandardDeviationPopulation(input Float64Data) (sdev float64, err error) {

	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}

	// Get the population variance
	vp, _ := PopulationVariance(input)

	// Return the population standard deviation
	return math.Sqrt(vp), nil
}

// StandardDeviationSample finds the amount of variation from a sample
func StandardDeviationSample(input Float64Data) (sdev float64, err error) {

	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}

	// Get the sample variance
	vs, _ := SampleVariance(input)

	// Return the sample standard deviation
	return math.Sqrt(vs), nil
}
