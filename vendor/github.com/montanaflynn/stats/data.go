package stats

// Float64Data is a named type for []float64 with helper methods
type Float64Data []float64

// Get item in slice
func (f Float64Data) Get(i int) float64 { return f[i] }

// Len returns length of slice
func (f Float64Data) Len() int { return len(f) }

// Less returns if one number is less than another
func (f Float64Data) Less(i, j int) bool { return f[i] < f[j] }

// Swap switches out two numbers in slice
func (f Float64Data) Swap(i, j int) { f[i], f[j] = f[j], f[i] }

// Min returns the minimum number in the data
func (f Float64Data) Min() (float64, error) { return Min(f) }

// Max returns the maximum number in the data
func (f Float64Data) Max() (float64, error) { return Max(f) }

// Sum returns the total of all the numbers in the data
func (f Float64Data) Sum() (float64, error) { return Sum(f) }

// CumulativeSum returns the cumulative sum of the data
func (f Float64Data) CumulativeSum() ([]float64, error) { return CumulativeSum(f) }

// Mean returns the mean of the data
func (f Float64Data) Mean() (float64, error) { return Mean(f) }

// Median returns the median of the data
func (f Float64Data) Median() (float64, error) { return Median(f) }

// Mode returns the mode of the data
func (f Float64Data) Mode() ([]float64, error) { return Mode(f) }

// GeometricMean returns the median of the data
func (f Float64Data) GeometricMean() (float64, error) { return GeometricMean(f) }

// HarmonicMean returns the mode of the data
func (f Float64Data) HarmonicMean() (float64, error) { return HarmonicMean(f) }

// MedianAbsoluteDeviation the median of the absolute deviations from the dataset median
func (f Float64Data) MedianAbsoluteDeviation() (float64, error) {
	return MedianAbsoluteDeviation(f)
}

// MedianAbsoluteDeviationPopulation finds the median of the absolute deviations from the population median
func (f Float64Data) MedianAbsoluteDeviationPopulation() (float64, error) {
	return MedianAbsoluteDeviationPopulation(f)
}

// StandardDeviation the amount of variation in the dataset
func (f Float64Data) StandardDeviation() (float64, error) {
	return StandardDeviation(f)
}

// StandardDeviationPopulation finds the amount of variation from the population
func (f Float64Data) StandardDeviationPopulation() (float64, error) {
	return StandardDeviationPopulation(f)
}

// StandardDeviationSample finds the amount of variation from a sample
func (f Float64Data) StandardDeviationSample() (float64, error) {
	return StandardDeviationSample(f)
}

// QuartileOutliers finds the mild and extreme outliers
func (f Float64Data) QuartileOutliers() (Outliers, error) {
	return QuartileOutliers(f)
}

// Percentile finds the relative standing in a slice of floats
func (f Float64Data) Percentile(p float64) (float64, error) {
	return Percentile(f, p)
}

// PercentileNearestRank finds the relative standing using the Nearest Rank method
func (f Float64Data) PercentileNearestRank(p float64) (float64, error) {
	return PercentileNearestRank(f, p)
}

// Correlation describes the degree of relationship between two sets of data
func (f Float64Data) Correlation(d Float64Data) (float64, error) {
	return Correlation(f, d)
}

// AutoCorrelation is the correlation of a signal with a delayed copy of itself as a function of delay
func (f Float64Data) AutoCorrelation(lags int) (float64, error) {
	return AutoCorrelation(f, lags)
}

// Pearson calculates the Pearson product-moment correlation coefficient between two variables.
func (f Float64Data) Pearson(d Float64Data) (float64, error) {
	return Pearson(f, d)
}

// Quartile returns the three quartile points from a slice of data
func (f Float64Data) Quartile(d Float64Data) (Quartiles, error) {
	return Quartile(d)
}

// InterQuartileRange finds the range between Q1 and Q3
func (f Float64Data) InterQuartileRange() (float64, error) {
	return InterQuartileRange(f)
}

// Midhinge finds the average of the first and third quartiles
func (f Float64Data) Midhinge(d Float64Data) (float64, error) {
	return Midhinge(d)
}

// Trimean finds the average of the median and the midhinge
func (f Float64Data) Trimean(d Float64Data) (float64, error) {
	return Trimean(d)
}

// Sample returns sample from input with replacement or without
func (f Float64Data) Sample(n int, r bool) ([]float64, error) {
	return Sample(f, n, r)
}

// Variance the amount of variation in the dataset
func (f Float64Data) Variance() (float64, error) {
	return Variance(f)
}

// PopulationVariance finds the amount of variance within a population
func (f Float64Data) PopulationVariance() (float64, error) {
	return PopulationVariance(f)
}

// SampleVariance finds the amount of variance within a sample
func (f Float64Data) SampleVariance() (float64, error) {
	return SampleVariance(f)
}

// Covariance is a measure of how much two sets of data change
func (f Float64Data) Covariance(d Float64Data) (float64, error) {
	return Covariance(f, d)
}

// CovariancePopulation computes covariance for entire population between two variables
func (f Float64Data) CovariancePopulation(d Float64Data) (float64, error) {
	return CovariancePopulation(f, d)
}

// Sigmoid returns the input values along the sigmoid or s-shaped curve
func (f Float64Data) Sigmoid() ([]float64, error) {
	return Sigmoid(f)
}

// SoftMax returns the input values in the range of 0 to 1
// with sum of all the probabilities being equal to one.
func (f Float64Data) SoftMax() ([]float64, error) {
	return SoftMax(f)
}

// Entropy provides calculation of the entropy
func (f Float64Data) Entropy() (float64, error) {
	return Entropy(f)
}

// Quartiles returns the three quartile points from instance of Float64Data
func (f Float64Data) Quartiles() (Quartiles, error) {
	return Quartile(f)
}
