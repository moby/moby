package stats

import (
	"math"
)

// Correlation describes the degree of relationship between two sets of data
func Correlation(data1, data2 Float64Data) (float64, error) {

	l1 := data1.Len()
	l2 := data2.Len()

	if l1 == 0 || l2 == 0 {
		return math.NaN(), EmptyInputErr
	}

	if l1 != l2 {
		return math.NaN(), SizeErr
	}

	sdev1, _ := StandardDeviationPopulation(data1)
	sdev2, _ := StandardDeviationPopulation(data2)

	if sdev1 == 0 || sdev2 == 0 {
		return 0, nil
	}

	covp, _ := CovariancePopulation(data1, data2)
	return covp / (sdev1 * sdev2), nil
}

// Pearson calculates the Pearson product-moment correlation coefficient between two variables
func Pearson(data1, data2 Float64Data) (float64, error) {
	return Correlation(data1, data2)
}

// AutoCorrelation is the correlation of a signal with a delayed copy of itself as a function of delay
func AutoCorrelation(data Float64Data, lags int) (float64, error) {
	if len(data) < 1 {
		return 0, EmptyInputErr
	}

	mean, _ := Mean(data)

	var result, q float64

	for i := 0; i < lags; i++ {
		v := (data[0] - mean) * (data[0] - mean)
		for i := 1; i < len(data); i++ {
			delta0 := data[i-1] - mean
			delta1 := data[i] - mean
			q += (delta0*delta1 - q) / float64(i+1)
			v += (delta1*delta1 - v) / float64(i+1)
		}

		result = q / v
	}

	return result, nil
}
