package stats

import "math"

// Quartiles holds the three quartile points
type Quartiles struct {
	Q1 float64
	Q2 float64
	Q3 float64
}

// Quartile returns the three quartile points from a slice of data
func Quartile(input Float64Data) (Quartiles, error) {

	il := input.Len()
	if il == 0 {
		return Quartiles{}, EmptyInputErr
	}

	// Start by sorting a copy of the slice
	copy := sortedCopy(input)

	// Find the cutoff places depeding on if
	// the input slice length is even or odd
	var c1 int
	var c2 int
	if il%2 == 0 {
		c1 = il / 2
		c2 = il / 2
	} else {
		c1 = (il - 1) / 2
		c2 = c1 + 1
	}

	// Find the Medians with the cutoff points
	Q1, _ := Median(copy[:c1])
	Q2, _ := Median(copy)
	Q3, _ := Median(copy[c2:])

	return Quartiles{Q1, Q2, Q3}, nil

}

// InterQuartileRange finds the range between Q1 and Q3
func InterQuartileRange(input Float64Data) (float64, error) {
	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}
	qs, _ := Quartile(input)
	iqr := qs.Q3 - qs.Q1
	return iqr, nil
}

// Midhinge finds the average of the first and third quartiles
func Midhinge(input Float64Data) (float64, error) {
	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}
	qs, _ := Quartile(input)
	mh := (qs.Q1 + qs.Q3) / 2
	return mh, nil
}

// Trimean finds the average of the median and the midhinge
func Trimean(input Float64Data) (float64, error) {
	if input.Len() == 0 {
		return math.NaN(), EmptyInputErr
	}

	c := sortedCopy(input)
	q, _ := Quartile(c)

	return (q.Q1 + (q.Q2 * 2) + q.Q3) / 4, nil
}
