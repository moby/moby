package stats

import "fmt"

// Holds information about the dataset provided to Describe
type Description struct {
	Count                  int
	Mean                   float64
	Std                    float64
	Max                    float64
	Min                    float64
	DescriptionPercentiles []descriptionPercentile
	AllowedNaN             bool
}

// Specifies percentiles to be computed
type descriptionPercentile struct {
	Percentile float64
	Value      float64
}

// Describe generates descriptive statistics about a provided dataset, similar to python's pandas.describe()
func Describe(input Float64Data, allowNaN bool, percentiles *[]float64) (*Description, error) {
	return DescribePercentileFunc(input, allowNaN, percentiles, Percentile)
}

// Describe generates descriptive statistics about a provided dataset, similar to python's pandas.describe()
// Takes in a function to use for percentile calculation
func DescribePercentileFunc(input Float64Data, allowNaN bool, percentiles *[]float64, percentileFunc func(Float64Data, float64) (float64, error)) (*Description, error) {
	var description Description
	description.AllowedNaN = allowNaN
	description.Count = input.Len()

	if description.Count == 0 && !allowNaN {
		return &description, ErrEmptyInput
	}

	// Disregard error, since it cannot be thrown if Count is > 0 and allowNaN is false, else NaN is accepted
	description.Std, _ = StandardDeviation(input)
	description.Max, _ = Max(input)
	description.Min, _ = Min(input)
	description.Mean, _ = Mean(input)

	if percentiles != nil {
		for _, percentile := range *percentiles {
			if value, err := percentileFunc(input, percentile); err == nil || allowNaN {
				description.DescriptionPercentiles = append(description.DescriptionPercentiles, descriptionPercentile{Percentile: percentile, Value: value})
			}
		}
	}

	return &description, nil
}

/*
Represents the Description instance in a string format with specified number of decimals

	count   3
	mean    2.00
	std     0.82
	max     3.00
	min     1.00
	25.00%  NaN
	50.00%  1.50
	75.00%  2.50
	NaN OK  true
*/
func (d *Description) String(decimals int) string {
	var str string

	str += fmt.Sprintf("count\t%d\n", d.Count)
	str += fmt.Sprintf("mean\t%.*f\n", decimals, d.Mean)
	str += fmt.Sprintf("std\t%.*f\n", decimals, d.Std)
	str += fmt.Sprintf("max\t%.*f\n", decimals, d.Max)
	str += fmt.Sprintf("min\t%.*f\n", decimals, d.Min)
	for _, percentile := range d.DescriptionPercentiles {
		str += fmt.Sprintf("%.2f%%\t%.*f\n", percentile.Percentile, decimals, percentile.Value)
	}
	str += fmt.Sprintf("NaN OK\t%t", d.AllowedNaN)
	return str
}
