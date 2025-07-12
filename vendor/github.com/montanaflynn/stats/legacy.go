package stats

// VarP is a shortcut to PopulationVariance
func VarP(input Float64Data) (sdev float64, err error) {
	return PopulationVariance(input)
}

// VarS is a shortcut to SampleVariance
func VarS(input Float64Data) (sdev float64, err error) {
	return SampleVariance(input)
}

// StdDevP is a shortcut to StandardDeviationPopulation
func StdDevP(input Float64Data) (sdev float64, err error) {
	return StandardDeviationPopulation(input)
}

// StdDevS is a shortcut to StandardDeviationSample
func StdDevS(input Float64Data) (sdev float64, err error) {
	return StandardDeviationSample(input)
}

// LinReg is a shortcut to LinearRegression
func LinReg(s []Coordinate) (regressions []Coordinate, err error) {
	return LinearRegression(s)
}

// ExpReg is a shortcut to ExponentialRegression
func ExpReg(s []Coordinate) (regressions []Coordinate, err error) {
	return ExponentialRegression(s)
}

// LogReg is a shortcut to LogarithmicRegression
func LogReg(s []Coordinate) (regressions []Coordinate, err error) {
	return LogarithmicRegression(s)
}

// Legacy error names that didn't start with Err
var (
	EmptyInputErr = ErrEmptyInput
	NaNErr        = ErrNaN
	NegativeErr   = ErrNegative
	ZeroErr       = ErrZero
	BoundsErr     = ErrBounds
	SizeErr       = ErrSize
	InfValue      = ErrInfValue
	YCoordErr     = ErrYCoord
	EmptyInput    = ErrEmptyInput
)
