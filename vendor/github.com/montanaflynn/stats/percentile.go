package stats

import (
	"math"
)

// Percentile finds the relative standing in a slice of floats.
//
// The function uses the Linear Interpolation Between Closest Ranks method
// as recommended by NIST [1] and used by Excel (PERCENTILE), Google Sheets,
// NumPy (default), and other standard tools.
//
// Algorithm (for percent p and sorted data of length n):
//
//  1. Compute the rank: rank = (p / 100) * (n - 1)
//  2. Split into integer part k and fractional part f
//  3. Result = data[k] + f * (data[k+1] - data[k])
//
// [1] https://www.itl.nist.gov/div898/handbook/prc/section2/prc262.htm
func Percentile(input Float64Data, percent float64) (percentile float64, err error) {
	length := input.Len()
	if length == 0 {
		return math.NaN(), EmptyInputErr
	}

	if length == 1 {
		return input[0], nil
	}

	if percent <= 0 || percent > 100 {
		return math.NaN(), BoundsErr
	}

	// Start by sorting a copy of the slice
	c := sortedCopy(input)

	// Use the standard linear interpolation method:
	// rank = (percent / 100) * (n - 1)
	// result = c[k] + f * (c[k+1] - c[k])
	rank := (percent / 100) * float64(length-1)
	k := int(rank)
	f := rank - float64(k)

	if k+1 < length {
		percentile = c[k] + f*(c[k+1]-c[k])
	} else {
		percentile = c[k]
	}

	return percentile, nil

}

// PercentileNearestRank finds the relative standing in a slice of floats using the Nearest Rank method
func PercentileNearestRank(input Float64Data, percent float64) (percentile float64, err error) {

	// Find the length of items in the slice
	il := input.Len()

	// Return an error for empty slices
	if il == 0 {
		return math.NaN(), EmptyInputErr
	}

	// Return error for less than 0 or greater than 100 percentages
	if percent < 0 || percent > 100 {
		return math.NaN(), BoundsErr
	}

	// Start by sorting a copy of the slice
	c := sortedCopy(input)

	// Return the last item
	if percent == 100.0 {
		return c[il-1], nil
	}

	// Find ordinal ranking
	or := int(math.Ceil(float64(il) * percent / 100))

	// Return the item that is in the place of the ordinal rank
	if or == 0 {
		return c[0], nil
	}
	return c[or-1], nil

}
