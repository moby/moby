package stats

import "math"

// Median gets the median number in a slice of numbers
func Median(input Float64Data) (median float64, err error) {

	// Start by sorting a copy of the slice
	c := sortedCopy(input)

	// No math is needed if there are no numbers
	// For even numbers we add the two middle numbers
	// and divide by two using the mean function above
	// For odd numbers we just use the middle number
	l := len(c)
	if l == 0 {
		return math.NaN(), EmptyInputErr
	} else if l%2 == 0 {
		median, _ = Mean(c[l/2-1 : l/2+1])
	} else {
		median = c[l/2]
	}

	return median, nil
}
