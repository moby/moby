package stats

// Outliers holds mild and extreme outliers found in data
type Outliers struct {
	Mild    Float64Data
	Extreme Float64Data
}

// QuartileOutliers finds the mild and extreme outliers
func QuartileOutliers(input Float64Data) (Outliers, error) {
	if input.Len() == 0 {
		return Outliers{}, EmptyInputErr
	}

	// Start by sorting a copy of the slice
	copy := sortedCopy(input)

	// Calculate the quartiles and interquartile range
	qs, _ := Quartile(copy)
	iqr, _ := InterQuartileRange(copy)

	// Calculate the lower and upper inner and outer fences
	lif := qs.Q1 - (1.5 * iqr)
	uif := qs.Q3 + (1.5 * iqr)
	lof := qs.Q1 - (3 * iqr)
	uof := qs.Q3 + (3 * iqr)

	// Find the data points that are outside of the
	// inner and upper fences and add them to mild
	// and extreme outlier slices
	var mild Float64Data
	var extreme Float64Data
	for _, v := range copy {

		if v < lof || v > uof {
			extreme = append(extreme, v)
		} else if v < lif || v > uif {
			mild = append(mild, v)
		}
	}

	// Wrap them into our struct
	return Outliers{mild, extreme}, nil
}
