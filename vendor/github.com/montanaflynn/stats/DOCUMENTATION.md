

# stats
`import "github.com/montanaflynn/stats"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Examples](#pkg-examples)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-overview">Overview</a>
Package stats is a well tested and comprehensive
statistics library package with no dependencies.

Example Usage:


	// start with some source data to use
	data := []float64{1.0, 2.1, 3.2, 4.823, 4.1, 5.8}
	
	// you could also use different types like this
	// data := stats.LoadRawData([]int{1, 2, 3, 4, 5})
	// data := stats.LoadRawData([]interface{}{1.1, "2", 3})
	// etc...
	
	median, _ := stats.Median(data)
	fmt.Println(median) // 3.65
	
	roundedMedian, _ := stats.Round(median, 0)
	fmt.Println(roundedMedian) // 4

MIT License Copyright (c) 2014-2020 Montana Flynn (<a href="https://montanaflynn.com">https://montanaflynn.com</a>)




## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [func AutoCorrelation(data Float64Data, lags int) (float64, error)](#AutoCorrelation)
* [func ChebyshevDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)](#ChebyshevDistance)
* [func Correlation(data1, data2 Float64Data) (float64, error)](#Correlation)
* [func Covariance(data1, data2 Float64Data) (float64, error)](#Covariance)
* [func CovariancePopulation(data1, data2 Float64Data) (float64, error)](#CovariancePopulation)
* [func CumulativeSum(input Float64Data) ([]float64, error)](#CumulativeSum)
* [func Entropy(input Float64Data) (float64, error)](#Entropy)
* [func EuclideanDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)](#EuclideanDistance)
* [func ExpGeom(p float64) (exp float64, err error)](#ExpGeom)
* [func GeometricMean(input Float64Data) (float64, error)](#GeometricMean)
* [func HarmonicMean(input Float64Data) (float64, error)](#HarmonicMean)
* [func InterQuartileRange(input Float64Data) (float64, error)](#InterQuartileRange)
* [func ManhattanDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)](#ManhattanDistance)
* [func Max(input Float64Data) (max float64, err error)](#Max)
* [func Mean(input Float64Data) (float64, error)](#Mean)
* [func Median(input Float64Data) (median float64, err error)](#Median)
* [func MedianAbsoluteDeviation(input Float64Data) (mad float64, err error)](#MedianAbsoluteDeviation)
* [func MedianAbsoluteDeviationPopulation(input Float64Data) (mad float64, err error)](#MedianAbsoluteDeviationPopulation)
* [func Midhinge(input Float64Data) (float64, error)](#Midhinge)
* [func Min(input Float64Data) (min float64, err error)](#Min)
* [func MinkowskiDistance(dataPointX, dataPointY Float64Data, lambda float64) (distance float64, err error)](#MinkowskiDistance)
* [func Mode(input Float64Data) (mode []float64, err error)](#Mode)
* [func Ncr(n, r int) int](#Ncr)
* [func NormBoxMullerRvs(loc float64, scale float64, size int) []float64](#NormBoxMullerRvs)
* [func NormCdf(x float64, loc float64, scale float64) float64](#NormCdf)
* [func NormEntropy(loc float64, scale float64) float64](#NormEntropy)
* [func NormFit(data []float64) [2]float64](#NormFit)
* [func NormInterval(alpha float64, loc float64, scale float64) [2]float64](#NormInterval)
* [func NormIsf(p float64, loc float64, scale float64) (x float64)](#NormIsf)
* [func NormLogCdf(x float64, loc float64, scale float64) float64](#NormLogCdf)
* [func NormLogPdf(x float64, loc float64, scale float64) float64](#NormLogPdf)
* [func NormLogSf(x float64, loc float64, scale float64) float64](#NormLogSf)
* [func NormMean(loc float64, scale float64) float64](#NormMean)
* [func NormMedian(loc float64, scale float64) float64](#NormMedian)
* [func NormMoment(n int, loc float64, scale float64) float64](#NormMoment)
* [func NormPdf(x float64, loc float64, scale float64) float64](#NormPdf)
* [func NormPpf(p float64, loc float64, scale float64) (x float64)](#NormPpf)
* [func NormPpfRvs(loc float64, scale float64, size int) []float64](#NormPpfRvs)
* [func NormSf(x float64, loc float64, scale float64) float64](#NormSf)
* [func NormStats(loc float64, scale float64, moments string) []float64](#NormStats)
* [func NormStd(loc float64, scale float64) float64](#NormStd)
* [func NormVar(loc float64, scale float64) float64](#NormVar)
* [func Pearson(data1, data2 Float64Data) (float64, error)](#Pearson)
* [func Percentile(input Float64Data, percent float64) (percentile float64, err error)](#Percentile)
* [func PercentileNearestRank(input Float64Data, percent float64) (percentile float64, err error)](#PercentileNearestRank)
* [func PopulationVariance(input Float64Data) (pvar float64, err error)](#PopulationVariance)
* [func ProbGeom(a int, b int, p float64) (prob float64, err error)](#ProbGeom)
* [func Round(input float64, places int) (rounded float64, err error)](#Round)
* [func Sample(input Float64Data, takenum int, replacement bool) ([]float64, error)](#Sample)
* [func SampleVariance(input Float64Data) (svar float64, err error)](#SampleVariance)
* [func Sigmoid(input Float64Data) ([]float64, error)](#Sigmoid)
* [func SoftMax(input Float64Data) ([]float64, error)](#SoftMax)
* [func StableSample(input Float64Data, takenum int) ([]float64, error)](#StableSample)
* [func StandardDeviation(input Float64Data) (sdev float64, err error)](#StandardDeviation)
* [func StandardDeviationPopulation(input Float64Data) (sdev float64, err error)](#StandardDeviationPopulation)
* [func StandardDeviationSample(input Float64Data) (sdev float64, err error)](#StandardDeviationSample)
* [func StdDevP(input Float64Data) (sdev float64, err error)](#StdDevP)
* [func StdDevS(input Float64Data) (sdev float64, err error)](#StdDevS)
* [func Sum(input Float64Data) (sum float64, err error)](#Sum)
* [func Trimean(input Float64Data) (float64, error)](#Trimean)
* [func VarGeom(p float64) (exp float64, err error)](#VarGeom)
* [func VarP(input Float64Data) (sdev float64, err error)](#VarP)
* [func VarS(input Float64Data) (sdev float64, err error)](#VarS)
* [func Variance(input Float64Data) (sdev float64, err error)](#Variance)
* [type Coordinate](#Coordinate)
  * [func ExpReg(s []Coordinate) (regressions []Coordinate, err error)](#ExpReg)
  * [func LinReg(s []Coordinate) (regressions []Coordinate, err error)](#LinReg)
  * [func LogReg(s []Coordinate) (regressions []Coordinate, err error)](#LogReg)
* [type Float64Data](#Float64Data)
  * [func LoadRawData(raw interface{}) (f Float64Data)](#LoadRawData)
  * [func (f Float64Data) AutoCorrelation(lags int) (float64, error)](#Float64Data.AutoCorrelation)
  * [func (f Float64Data) Correlation(d Float64Data) (float64, error)](#Float64Data.Correlation)
  * [func (f Float64Data) Covariance(d Float64Data) (float64, error)](#Float64Data.Covariance)
  * [func (f Float64Data) CovariancePopulation(d Float64Data) (float64, error)](#Float64Data.CovariancePopulation)
  * [func (f Float64Data) CumulativeSum() ([]float64, error)](#Float64Data.CumulativeSum)
  * [func (f Float64Data) Entropy() (float64, error)](#Float64Data.Entropy)
  * [func (f Float64Data) GeometricMean() (float64, error)](#Float64Data.GeometricMean)
  * [func (f Float64Data) Get(i int) float64](#Float64Data.Get)
  * [func (f Float64Data) HarmonicMean() (float64, error)](#Float64Data.HarmonicMean)
  * [func (f Float64Data) InterQuartileRange() (float64, error)](#Float64Data.InterQuartileRange)
  * [func (f Float64Data) Len() int](#Float64Data.Len)
  * [func (f Float64Data) Less(i, j int) bool](#Float64Data.Less)
  * [func (f Float64Data) Max() (float64, error)](#Float64Data.Max)
  * [func (f Float64Data) Mean() (float64, error)](#Float64Data.Mean)
  * [func (f Float64Data) Median() (float64, error)](#Float64Data.Median)
  * [func (f Float64Data) MedianAbsoluteDeviation() (float64, error)](#Float64Data.MedianAbsoluteDeviation)
  * [func (f Float64Data) MedianAbsoluteDeviationPopulation() (float64, error)](#Float64Data.MedianAbsoluteDeviationPopulation)
  * [func (f Float64Data) Midhinge(d Float64Data) (float64, error)](#Float64Data.Midhinge)
  * [func (f Float64Data) Min() (float64, error)](#Float64Data.Min)
  * [func (f Float64Data) Mode() ([]float64, error)](#Float64Data.Mode)
  * [func (f Float64Data) Pearson(d Float64Data) (float64, error)](#Float64Data.Pearson)
  * [func (f Float64Data) Percentile(p float64) (float64, error)](#Float64Data.Percentile)
  * [func (f Float64Data) PercentileNearestRank(p float64) (float64, error)](#Float64Data.PercentileNearestRank)
  * [func (f Float64Data) PopulationVariance() (float64, error)](#Float64Data.PopulationVariance)
  * [func (f Float64Data) Quartile(d Float64Data) (Quartiles, error)](#Float64Data.Quartile)
  * [func (f Float64Data) QuartileOutliers() (Outliers, error)](#Float64Data.QuartileOutliers)
  * [func (f Float64Data) Quartiles() (Quartiles, error)](#Float64Data.Quartiles)
  * [func (f Float64Data) Sample(n int, r bool) ([]float64, error)](#Float64Data.Sample)
  * [func (f Float64Data) SampleVariance() (float64, error)](#Float64Data.SampleVariance)
  * [func (f Float64Data) Sigmoid() ([]float64, error)](#Float64Data.Sigmoid)
  * [func (f Float64Data) SoftMax() ([]float64, error)](#Float64Data.SoftMax)
  * [func (f Float64Data) StandardDeviation() (float64, error)](#Float64Data.StandardDeviation)
  * [func (f Float64Data) StandardDeviationPopulation() (float64, error)](#Float64Data.StandardDeviationPopulation)
  * [func (f Float64Data) StandardDeviationSample() (float64, error)](#Float64Data.StandardDeviationSample)
  * [func (f Float64Data) Sum() (float64, error)](#Float64Data.Sum)
  * [func (f Float64Data) Swap(i, j int)](#Float64Data.Swap)
  * [func (f Float64Data) Trimean(d Float64Data) (float64, error)](#Float64Data.Trimean)
  * [func (f Float64Data) Variance() (float64, error)](#Float64Data.Variance)
* [type Outliers](#Outliers)
  * [func QuartileOutliers(input Float64Data) (Outliers, error)](#QuartileOutliers)
* [type Quartiles](#Quartiles)
  * [func Quartile(input Float64Data) (Quartiles, error)](#Quartile)
* [type Series](#Series)
  * [func ExponentialRegression(s Series) (regressions Series, err error)](#ExponentialRegression)
  * [func LinearRegression(s Series) (regressions Series, err error)](#LinearRegression)
  * [func LogarithmicRegression(s Series) (regressions Series, err error)](#LogarithmicRegression)

#### <a name="pkg-examples">Examples</a>
* [AutoCorrelation](#example_AutoCorrelation)
* [ChebyshevDistance](#example_ChebyshevDistance)
* [Correlation](#example_Correlation)
* [CumulativeSum](#example_CumulativeSum)
* [Entropy](#example_Entropy)
* [ExpGeom](#example_ExpGeom)
* [LinearRegression](#example_LinearRegression)
* [LoadRawData](#example_LoadRawData)
* [Max](#example_Max)
* [Median](#example_Median)
* [Min](#example_Min)
* [ProbGeom](#example_ProbGeom)
* [Round](#example_Round)
* [Sigmoid](#example_Sigmoid)
* [SoftMax](#example_SoftMax)
* [Sum](#example_Sum)
* [VarGeom](#example_VarGeom)

#### <a name="pkg-files">Package files</a>
[correlation.go](/src/github.com/montanaflynn/stats/correlation.go) [cumulative_sum.go](/src/github.com/montanaflynn/stats/cumulative_sum.go) [data.go](/src/github.com/montanaflynn/stats/data.go) [deviation.go](/src/github.com/montanaflynn/stats/deviation.go) [distances.go](/src/github.com/montanaflynn/stats/distances.go) [doc.go](/src/github.com/montanaflynn/stats/doc.go) [entropy.go](/src/github.com/montanaflynn/stats/entropy.go) [errors.go](/src/github.com/montanaflynn/stats/errors.go) [geometric_distribution.go](/src/github.com/montanaflynn/stats/geometric_distribution.go) [legacy.go](/src/github.com/montanaflynn/stats/legacy.go) [load.go](/src/github.com/montanaflynn/stats/load.go) [max.go](/src/github.com/montanaflynn/stats/max.go) [mean.go](/src/github.com/montanaflynn/stats/mean.go) [median.go](/src/github.com/montanaflynn/stats/median.go) [min.go](/src/github.com/montanaflynn/stats/min.go) [mode.go](/src/github.com/montanaflynn/stats/mode.go) [norm.go](/src/github.com/montanaflynn/stats/norm.go) [outlier.go](/src/github.com/montanaflynn/stats/outlier.go) [percentile.go](/src/github.com/montanaflynn/stats/percentile.go) [quartile.go](/src/github.com/montanaflynn/stats/quartile.go) [ranksum.go](/src/github.com/montanaflynn/stats/ranksum.go) [regression.go](/src/github.com/montanaflynn/stats/regression.go) [round.go](/src/github.com/montanaflynn/stats/round.go) [sample.go](/src/github.com/montanaflynn/stats/sample.go) [sigmoid.go](/src/github.com/montanaflynn/stats/sigmoid.go) [softmax.go](/src/github.com/montanaflynn/stats/softmax.go) [sum.go](/src/github.com/montanaflynn/stats/sum.go) [util.go](/src/github.com/montanaflynn/stats/util.go) [variance.go](/src/github.com/montanaflynn/stats/variance.go) 



## <a name="pkg-variables">Variables</a>
``` go
var (
    // ErrEmptyInput Input must not be empty
    ErrEmptyInput = statsError{"Input must not be empty."}
    // ErrNaN Not a number
    ErrNaN = statsError{"Not a number."}
    // ErrNegative Must not contain negative values
    ErrNegative = statsError{"Must not contain negative values."}
    // ErrZero Must not contain zero values
    ErrZero = statsError{"Must not contain zero values."}
    // ErrBounds Input is outside of range
    ErrBounds = statsError{"Input is outside of range."}
    // ErrSize Must be the same length
    ErrSize = statsError{"Must be the same length."}
    // ErrInfValue Value is infinite
    ErrInfValue = statsError{"Value is infinite."}
    // ErrYCoord Y Value must be greater than zero
    ErrYCoord = statsError{"Y Value must be greater than zero."}
)
```
These are the package-wide error values.
All error identification should use these values.
<a href="https://github.com/golang/go/wiki/Errors#naming">https://github.com/golang/go/wiki/Errors#naming</a>

``` go
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
```
Legacy error names that didn't start with Err



## <a name="AutoCorrelation">func</a> [AutoCorrelation](/correlation.go?s=853:918#L38)
``` go
func AutoCorrelation(data Float64Data, lags int) (float64, error)
```
AutoCorrelation is the correlation of a signal with a delayed copy of itself as a function of delay



## <a name="ChebyshevDistance">func</a> [ChebyshevDistance](/distances.go?s=368:456#L20)
``` go
func ChebyshevDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)
```
ChebyshevDistance computes the Chebyshev distance between two data sets



## <a name="Correlation">func</a> [Correlation](/correlation.go?s=112:171#L8)
``` go
func Correlation(data1, data2 Float64Data) (float64, error)
```
Correlation describes the degree of relationship between two sets of data



## <a name="Covariance">func</a> [Covariance](/variance.go?s=1284:1342#L53)
``` go
func Covariance(data1, data2 Float64Data) (float64, error)
```
Covariance is a measure of how much two sets of data change



## <a name="CovariancePopulation">func</a> [CovariancePopulation](/variance.go?s=1864:1932#L81)
``` go
func CovariancePopulation(data1, data2 Float64Data) (float64, error)
```
CovariancePopulation computes covariance for entire population between two variables.



## <a name="CumulativeSum">func</a> [CumulativeSum](/cumulative_sum.go?s=81:137#L4)
``` go
func CumulativeSum(input Float64Data) ([]float64, error)
```
CumulativeSum calculates the cumulative sum of the input slice



## <a name="Entropy">func</a> [Entropy](/entropy.go?s=77:125#L6)
``` go
func Entropy(input Float64Data) (float64, error)
```
Entropy provides calculation of the entropy



## <a name="EuclideanDistance">func</a> [EuclideanDistance](/distances.go?s=836:924#L36)
``` go
func EuclideanDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)
```
EuclideanDistance computes the Euclidean distance between two data sets



## <a name="ExpGeom">func</a> [ExpGeom](/geometric_distribution.go?s=652:700#L27)
``` go
func ExpGeom(p float64) (exp float64, err error)
```
ProbGeom generates the expectation or average number of trials
for a geometric random variable with parameter p



## <a name="GeometricMean">func</a> [GeometricMean](/mean.go?s=319:373#L18)
``` go
func GeometricMean(input Float64Data) (float64, error)
```
GeometricMean gets the geometric mean for a slice of numbers



## <a name="HarmonicMean">func</a> [HarmonicMean](/mean.go?s=717:770#L40)
``` go
func HarmonicMean(input Float64Data) (float64, error)
```
HarmonicMean gets the harmonic mean for a slice of numbers



## <a name="InterQuartileRange">func</a> [InterQuartileRange](/quartile.go?s=821:880#L45)
``` go
func InterQuartileRange(input Float64Data) (float64, error)
```
InterQuartileRange finds the range between Q1 and Q3



## <a name="ManhattanDistance">func</a> [ManhattanDistance](/distances.go?s=1277:1365#L50)
``` go
func ManhattanDistance(dataPointX, dataPointY Float64Data) (distance float64, err error)
```
ManhattanDistance computes the Manhattan distance between two data sets



## <a name="Max">func</a> [Max](/max.go?s=78:130#L8)
``` go
func Max(input Float64Data) (max float64, err error)
```
Max finds the highest number in a slice



## <a name="Mean">func</a> [Mean](/mean.go?s=77:122#L6)
``` go
func Mean(input Float64Data) (float64, error)
```
Mean gets the average of a slice of numbers



## <a name="Median">func</a> [Median](/median.go?s=85:143#L6)
``` go
func Median(input Float64Data) (median float64, err error)
```
Median gets the median number in a slice of numbers



## <a name="MedianAbsoluteDeviation">func</a> [MedianAbsoluteDeviation](/deviation.go?s=125:197#L6)
``` go
func MedianAbsoluteDeviation(input Float64Data) (mad float64, err error)
```
MedianAbsoluteDeviation finds the median of the absolute deviations from the dataset median



## <a name="MedianAbsoluteDeviationPopulation">func</a> [MedianAbsoluteDeviationPopulation](/deviation.go?s=360:442#L11)
``` go
func MedianAbsoluteDeviationPopulation(input Float64Data) (mad float64, err error)
```
MedianAbsoluteDeviationPopulation finds the median of the absolute deviations from the population median



## <a name="Midhinge">func</a> [Midhinge](/quartile.go?s=1075:1124#L55)
``` go
func Midhinge(input Float64Data) (float64, error)
```
Midhinge finds the average of the first and third quartiles



## <a name="Min">func</a> [Min](/min.go?s=78:130#L6)
``` go
func Min(input Float64Data) (min float64, err error)
```
Min finds the lowest number in a set of data



## <a name="MinkowskiDistance">func</a> [MinkowskiDistance](/distances.go?s=2152:2256#L75)
``` go
func MinkowskiDistance(dataPointX, dataPointY Float64Data, lambda float64) (distance float64, err error)
```
MinkowskiDistance computes the Minkowski distance between two data sets

Arguments:


	dataPointX: First set of data points
	dataPointY: Second set of data points. Length of both data
	            sets must be equal.
	lambda:     aka p or city blocks; With lambda = 1
	            returned distance is manhattan distance and
	            lambda = 2; it is euclidean distance. Lambda
	            reaching to infinite - distance would be chebysev
	            distance.

Return:


	Distance or error



## <a name="Mode">func</a> [Mode](/mode.go?s=85:141#L4)
``` go
func Mode(input Float64Data) (mode []float64, err error)
```
Mode gets the mode [most frequent value(s)] of a slice of float64s



## <a name="Ncr">func</a> [Ncr](/norm.go?s=7384:7406#L239)
``` go
func Ncr(n, r int) int
```
Ncr is an N choose R algorithm.
Aaron Cannon's algorithm.



## <a name="NormBoxMullerRvs">func</a> [NormBoxMullerRvs](/norm.go?s=667:736#L23)
``` go
func NormBoxMullerRvs(loc float64, scale float64, size int) []float64
```
NormBoxMullerRvs generates random variates using the Box–Muller transform.
For more information please visit: <a href="http://mathworld.wolfram.com/Box-MullerTransformation.html">http://mathworld.wolfram.com/Box-MullerTransformation.html</a>



## <a name="NormCdf">func</a> [NormCdf](/norm.go?s=1826:1885#L52)
``` go
func NormCdf(x float64, loc float64, scale float64) float64
```
NormCdf is the cumulative distribution function.



## <a name="NormEntropy">func</a> [NormEntropy](/norm.go?s=5773:5825#L180)
``` go
func NormEntropy(loc float64, scale float64) float64
```
NormEntropy is the differential entropy of the RV.



## <a name="NormFit">func</a> [NormFit](/norm.go?s=6058:6097#L187)
``` go
func NormFit(data []float64) [2]float64
```
NormFit returns the maximum likelihood estimators for the Normal Distribution.
Takes array of float64 values.
Returns array of Mean followed by Standard Deviation.



## <a name="NormInterval">func</a> [NormInterval](/norm.go?s=6976:7047#L221)
``` go
func NormInterval(alpha float64, loc float64, scale float64) [2]float64
```
NormInterval finds endpoints of the range that contains alpha percent of the distribution.



## <a name="NormIsf">func</a> [NormIsf](/norm.go?s=4330:4393#L137)
``` go
func NormIsf(p float64, loc float64, scale float64) (x float64)
```
NormIsf is the inverse survival function (inverse of sf).



## <a name="NormLogCdf">func</a> [NormLogCdf](/norm.go?s=2016:2078#L57)
``` go
func NormLogCdf(x float64, loc float64, scale float64) float64
```
NormLogCdf is the log of the cumulative distribution function.



## <a name="NormLogPdf">func</a> [NormLogPdf](/norm.go?s=1590:1652#L47)
``` go
func NormLogPdf(x float64, loc float64, scale float64) float64
```
NormLogPdf is the log of the probability density function.



## <a name="NormLogSf">func</a> [NormLogSf](/norm.go?s=2423:2484#L67)
``` go
func NormLogSf(x float64, loc float64, scale float64) float64
```
NormLogSf is the log of the survival function.



## <a name="NormMean">func</a> [NormMean](/norm.go?s=6560:6609#L206)
``` go
func NormMean(loc float64, scale float64) float64
```
NormMean is the mean/expected value of the distribution.



## <a name="NormMedian">func</a> [NormMedian](/norm.go?s=6431:6482#L201)
``` go
func NormMedian(loc float64, scale float64) float64
```
NormMedian is the median of the distribution.



## <a name="NormMoment">func</a> [NormMoment](/norm.go?s=4694:4752#L146)
``` go
func NormMoment(n int, loc float64, scale float64) float64
```
NormMoment approximates the non-central (raw) moment of order n.
For more information please visit: <a href="https://math.stackexchange.com/questions/1945448/methods-for-finding-raw-moments-of-the-normal-distribution">https://math.stackexchange.com/questions/1945448/methods-for-finding-raw-moments-of-the-normal-distribution</a>



## <a name="NormPdf">func</a> [NormPdf](/norm.go?s=1357:1416#L42)
``` go
func NormPdf(x float64, loc float64, scale float64) float64
```
NormPdf is the probability density function.



## <a name="NormPpf">func</a> [NormPpf](/norm.go?s=2854:2917#L75)
``` go
func NormPpf(p float64, loc float64, scale float64) (x float64)
```
NormPpf is the point percentile function.
This is based on Peter John Acklam's inverse normal CDF.
algorithm: <a href="http://home.online.no/~pjacklam/notes/invnorm/">http://home.online.no/~pjacklam/notes/invnorm/</a> (no longer visible).
For more information please visit: <a href="https://stackedboxes.org/2017/05/01/acklams-normal-quantile-function/">https://stackedboxes.org/2017/05/01/acklams-normal-quantile-function/</a>



## <a name="NormPpfRvs">func</a> [NormPpfRvs](/norm.go?s=247:310#L12)
``` go
func NormPpfRvs(loc float64, scale float64, size int) []float64
```
NormPpfRvs generates random variates using the Point Percentile Function.
For more information please visit: <a href="https://demonstrations.wolfram.com/TheMethodOfInverseTransforms/">https://demonstrations.wolfram.com/TheMethodOfInverseTransforms/</a>



## <a name="NormSf">func</a> [NormSf](/norm.go?s=2250:2308#L62)
``` go
func NormSf(x float64, loc float64, scale float64) float64
```
NormSf is the survival function (also defined as 1 - cdf, but sf is sometimes more accurate).



## <a name="NormStats">func</a> [NormStats](/norm.go?s=5277:5345#L162)
``` go
func NormStats(loc float64, scale float64, moments string) []float64
```
NormStats returns the mean, variance, skew, and/or kurtosis.
Mean(‘m’), variance(‘v’), skew(‘s’), and/or kurtosis(‘k’).
Takes string containing any of 'mvsk'.
Returns array of m v s k in that order.



## <a name="NormStd">func</a> [NormStd](/norm.go?s=6814:6862#L216)
``` go
func NormStd(loc float64, scale float64) float64
```
NormStd is the standard deviation of the distribution.



## <a name="NormVar">func</a> [NormVar](/norm.go?s=6675:6723#L211)
``` go
func NormVar(loc float64, scale float64) float64
```
NormVar is the variance of the distribution.



## <a name="Pearson">func</a> [Pearson](/correlation.go?s=655:710#L33)
``` go
func Pearson(data1, data2 Float64Data) (float64, error)
```
Pearson calculates the Pearson product-moment correlation coefficient between two variables



## <a name="Percentile">func</a> [Percentile](/percentile.go?s=98:181#L8)
``` go
func Percentile(input Float64Data, percent float64) (percentile float64, err error)
```
Percentile finds the relative standing in a slice of floats



## <a name="PercentileNearestRank">func</a> [PercentileNearestRank](/percentile.go?s=1079:1173#L54)
``` go
func PercentileNearestRank(input Float64Data, percent float64) (percentile float64, err error)
```
PercentileNearestRank finds the relative standing in a slice of floats using the Nearest Rank method



## <a name="PopulationVariance">func</a> [PopulationVariance](/variance.go?s=828:896#L31)
``` go
func PopulationVariance(input Float64Data) (pvar float64, err error)
```
PopulationVariance finds the amount of variance within a population



## <a name="ProbGeom">func</a> [ProbGeom](/geometric_distribution.go?s=258:322#L10)
``` go
func ProbGeom(a int, b int, p float64) (prob float64, err error)
```
ProbGeom generates the probability for a geometric random variable
with parameter p to achieve success in the interval of [a, b] trials
See <a href="https://en.wikipedia.org/wiki/Geometric_distribution">https://en.wikipedia.org/wiki/Geometric_distribution</a> for more information



## <a name="Round">func</a> [Round](/round.go?s=88:154#L6)
``` go
func Round(input float64, places int) (rounded float64, err error)
```
Round a float to a specific decimal place or precision



## <a name="Sample">func</a> [Sample](/sample.go?s=112:192#L9)
``` go
func Sample(input Float64Data, takenum int, replacement bool) ([]float64, error)
```
Sample returns sample from input with replacement or without



## <a name="SampleVariance">func</a> [SampleVariance](/variance.go?s=1058:1122#L42)
``` go
func SampleVariance(input Float64Data) (svar float64, err error)
```
SampleVariance finds the amount of variance within a sample



## <a name="Sigmoid">func</a> [Sigmoid](/sigmoid.go?s=228:278#L9)
``` go
func Sigmoid(input Float64Data) ([]float64, error)
```
Sigmoid returns the input values in the range of -1 to 1
along the sigmoid or s-shaped curve, commonly used in
machine learning while training neural networks as an
activation function.



## <a name="SoftMax">func</a> [SoftMax](/softmax.go?s=206:256#L8)
``` go
func SoftMax(input Float64Data) ([]float64, error)
```
SoftMax returns the input values in the range of 0 to 1
with sum of all the probabilities being equal to one. It
is commonly used in machine learning neural networks.



## <a name="StableSample">func</a> [StableSample](/sample.go?s=974:1042#L50)
``` go
func StableSample(input Float64Data, takenum int) ([]float64, error)
```
StableSample like stable sort, it returns samples from input while keeps the order of original data.



## <a name="StandardDeviation">func</a> [StandardDeviation](/deviation.go?s=695:762#L27)
``` go
func StandardDeviation(input Float64Data) (sdev float64, err error)
```
StandardDeviation the amount of variation in the dataset



## <a name="StandardDeviationPopulation">func</a> [StandardDeviationPopulation](/deviation.go?s=892:969#L32)
``` go
func StandardDeviationPopulation(input Float64Data) (sdev float64, err error)
```
StandardDeviationPopulation finds the amount of variation from the population



## <a name="StandardDeviationSample">func</a> [StandardDeviationSample](/deviation.go?s=1250:1323#L46)
``` go
func StandardDeviationSample(input Float64Data) (sdev float64, err error)
```
StandardDeviationSample finds the amount of variation from a sample



## <a name="StdDevP">func</a> [StdDevP](/legacy.go?s=339:396#L14)
``` go
func StdDevP(input Float64Data) (sdev float64, err error)
```
StdDevP is a shortcut to StandardDeviationPopulation



## <a name="StdDevS">func</a> [StdDevS](/legacy.go?s=497:554#L19)
``` go
func StdDevS(input Float64Data) (sdev float64, err error)
```
StdDevS is a shortcut to StandardDeviationSample



## <a name="Sum">func</a> [Sum](/sum.go?s=78:130#L6)
``` go
func Sum(input Float64Data) (sum float64, err error)
```
Sum adds all the numbers of a slice together



## <a name="Trimean">func</a> [Trimean](/quartile.go?s=1320:1368#L65)
``` go
func Trimean(input Float64Data) (float64, error)
```
Trimean finds the average of the median and the midhinge



## <a name="VarGeom">func</a> [VarGeom](/geometric_distribution.go?s=885:933#L37)
``` go
func VarGeom(p float64) (exp float64, err error)
```
ProbGeom generates the variance for number for a
geometric random variable with parameter p



## <a name="VarP">func</a> [VarP](/legacy.go?s=59:113#L4)
``` go
func VarP(input Float64Data) (sdev float64, err error)
```
VarP is a shortcut to PopulationVariance



## <a name="VarS">func</a> [VarS](/legacy.go?s=193:247#L9)
``` go
func VarS(input Float64Data) (sdev float64, err error)
```
VarS is a shortcut to SampleVariance



## <a name="Variance">func</a> [Variance](/variance.go?s=659:717#L26)
``` go
func Variance(input Float64Data) (sdev float64, err error)
```
Variance the amount of variation in the dataset




## <a name="Coordinate">type</a> [Coordinate](/regression.go?s=143:183#L9)
``` go
type Coordinate struct {
    X, Y float64
}

```
Coordinate holds the data in a series







### <a name="ExpReg">func</a> [ExpReg](/legacy.go?s=791:856#L29)
``` go
func ExpReg(s []Coordinate) (regressions []Coordinate, err error)
```
ExpReg is a shortcut to ExponentialRegression


### <a name="LinReg">func</a> [LinReg](/legacy.go?s=643:708#L24)
``` go
func LinReg(s []Coordinate) (regressions []Coordinate, err error)
```
LinReg is a shortcut to LinearRegression


### <a name="LogReg">func</a> [LogReg](/legacy.go?s=944:1009#L34)
``` go
func LogReg(s []Coordinate) (regressions []Coordinate, err error)
```
LogReg is a shortcut to LogarithmicRegression





## <a name="Float64Data">type</a> [Float64Data](/data.go?s=80:106#L4)
``` go
type Float64Data []float64
```
Float64Data is a named type for []float64 with helper methods







### <a name="LoadRawData">func</a> [LoadRawData](/load.go?s=145:194#L12)
``` go
func LoadRawData(raw interface{}) (f Float64Data)
```
LoadRawData parses and converts a slice of mixed data types to floats





### <a name="Float64Data.AutoCorrelation">func</a> (Float64Data) [AutoCorrelation](/data.go?s=3257:3320#L91)
``` go
func (f Float64Data) AutoCorrelation(lags int) (float64, error)
```
AutoCorrelation is the correlation of a signal with a delayed copy of itself as a function of delay




### <a name="Float64Data.Correlation">func</a> (Float64Data) [Correlation](/data.go?s=3058:3122#L86)
``` go
func (f Float64Data) Correlation(d Float64Data) (float64, error)
```
Correlation describes the degree of relationship between two sets of data




### <a name="Float64Data.Covariance">func</a> (Float64Data) [Covariance](/data.go?s=4801:4864#L141)
``` go
func (f Float64Data) Covariance(d Float64Data) (float64, error)
```
Covariance is a measure of how much two sets of data change




### <a name="Float64Data.CovariancePopulation">func</a> (Float64Data) [CovariancePopulation](/data.go?s=4983:5056#L146)
``` go
func (f Float64Data) CovariancePopulation(d Float64Data) (float64, error)
```
CovariancePopulation computes covariance for entire population between two variables




### <a name="Float64Data.CumulativeSum">func</a> (Float64Data) [CumulativeSum](/data.go?s=883:938#L28)
``` go
func (f Float64Data) CumulativeSum() ([]float64, error)
```
CumulativeSum returns the cumulative sum of the data




### <a name="Float64Data.Entropy">func</a> (Float64Data) [Entropy](/data.go?s=5480:5527#L162)
``` go
func (f Float64Data) Entropy() (float64, error)
```
Entropy provides calculation of the entropy




### <a name="Float64Data.GeometricMean">func</a> (Float64Data) [GeometricMean](/data.go?s=1332:1385#L40)
``` go
func (f Float64Data) GeometricMean() (float64, error)
```
GeometricMean returns the median of the data




### <a name="Float64Data.Get">func</a> (Float64Data) [Get](/data.go?s=129:168#L7)
``` go
func (f Float64Data) Get(i int) float64
```
Get item in slice




### <a name="Float64Data.HarmonicMean">func</a> (Float64Data) [HarmonicMean](/data.go?s=1460:1512#L43)
``` go
func (f Float64Data) HarmonicMean() (float64, error)
```
HarmonicMean returns the mode of the data




### <a name="Float64Data.InterQuartileRange">func</a> (Float64Data) [InterQuartileRange](/data.go?s=3755:3813#L106)
``` go
func (f Float64Data) InterQuartileRange() (float64, error)
```
InterQuartileRange finds the range between Q1 and Q3




### <a name="Float64Data.Len">func</a> (Float64Data) [Len](/data.go?s=217:247#L10)
``` go
func (f Float64Data) Len() int
```
Len returns length of slice




### <a name="Float64Data.Less">func</a> (Float64Data) [Less](/data.go?s=318:358#L13)
``` go
func (f Float64Data) Less(i, j int) bool
```
Less returns if one number is less than another




### <a name="Float64Data.Max">func</a> (Float64Data) [Max](/data.go?s=645:688#L22)
``` go
func (f Float64Data) Max() (float64, error)
```
Max returns the maximum number in the data




### <a name="Float64Data.Mean">func</a> (Float64Data) [Mean](/data.go?s=1005:1049#L31)
``` go
func (f Float64Data) Mean() (float64, error)
```
Mean returns the mean of the data




### <a name="Float64Data.Median">func</a> (Float64Data) [Median](/data.go?s=1111:1157#L34)
``` go
func (f Float64Data) Median() (float64, error)
```
Median returns the median of the data




### <a name="Float64Data.MedianAbsoluteDeviation">func</a> (Float64Data) [MedianAbsoluteDeviation](/data.go?s=1630:1693#L46)
``` go
func (f Float64Data) MedianAbsoluteDeviation() (float64, error)
```
MedianAbsoluteDeviation the median of the absolute deviations from the dataset median




### <a name="Float64Data.MedianAbsoluteDeviationPopulation">func</a> (Float64Data) [MedianAbsoluteDeviationPopulation](/data.go?s=1842:1915#L51)
``` go
func (f Float64Data) MedianAbsoluteDeviationPopulation() (float64, error)
```
MedianAbsoluteDeviationPopulation finds the median of the absolute deviations from the population median




### <a name="Float64Data.Midhinge">func</a> (Float64Data) [Midhinge](/data.go?s=3912:3973#L111)
``` go
func (f Float64Data) Midhinge(d Float64Data) (float64, error)
```
Midhinge finds the average of the first and third quartiles




### <a name="Float64Data.Min">func</a> (Float64Data) [Min](/data.go?s=536:579#L19)
``` go
func (f Float64Data) Min() (float64, error)
```
Min returns the minimum number in the data




### <a name="Float64Data.Mode">func</a> (Float64Data) [Mode](/data.go?s=1217:1263#L37)
``` go
func (f Float64Data) Mode() ([]float64, error)
```
Mode returns the mode of the data




### <a name="Float64Data.Pearson">func</a> (Float64Data) [Pearson](/data.go?s=3455:3515#L96)
``` go
func (f Float64Data) Pearson(d Float64Data) (float64, error)
```
Pearson calculates the Pearson product-moment correlation coefficient between two variables.




### <a name="Float64Data.Percentile">func</a> (Float64Data) [Percentile](/data.go?s=2696:2755#L76)
``` go
func (f Float64Data) Percentile(p float64) (float64, error)
```
Percentile finds the relative standing in a slice of floats




### <a name="Float64Data.PercentileNearestRank">func</a> (Float64Data) [PercentileNearestRank](/data.go?s=2869:2939#L81)
``` go
func (f Float64Data) PercentileNearestRank(p float64) (float64, error)
```
PercentileNearestRank finds the relative standing using the Nearest Rank method




### <a name="Float64Data.PopulationVariance">func</a> (Float64Data) [PopulationVariance](/data.go?s=4495:4553#L131)
``` go
func (f Float64Data) PopulationVariance() (float64, error)
```
PopulationVariance finds the amount of variance within a population




### <a name="Float64Data.Quartile">func</a> (Float64Data) [Quartile](/data.go?s=3610:3673#L101)
``` go
func (f Float64Data) Quartile(d Float64Data) (Quartiles, error)
```
Quartile returns the three quartile points from a slice of data




### <a name="Float64Data.QuartileOutliers">func</a> (Float64Data) [QuartileOutliers](/data.go?s=2542:2599#L71)
``` go
func (f Float64Data) QuartileOutliers() (Outliers, error)
```
QuartileOutliers finds the mild and extreme outliers




### <a name="Float64Data.Quartiles">func</a> (Float64Data) [Quartiles](/data.go?s=5628:5679#L167)
``` go
func (f Float64Data) Quartiles() (Quartiles, error)
```
Quartiles returns the three quartile points from instance of Float64Data




### <a name="Float64Data.Sample">func</a> (Float64Data) [Sample](/data.go?s=4208:4269#L121)
``` go
func (f Float64Data) Sample(n int, r bool) ([]float64, error)
```
Sample returns sample from input with replacement or without




### <a name="Float64Data.SampleVariance">func</a> (Float64Data) [SampleVariance](/data.go?s=4652:4706#L136)
``` go
func (f Float64Data) SampleVariance() (float64, error)
```
SampleVariance finds the amount of variance within a sample




### <a name="Float64Data.Sigmoid">func</a> (Float64Data) [Sigmoid](/data.go?s=5169:5218#L151)
``` go
func (f Float64Data) Sigmoid() ([]float64, error)
```
Sigmoid returns the input values along the sigmoid or s-shaped curve




### <a name="Float64Data.SoftMax">func</a> (Float64Data) [SoftMax](/data.go?s=5359:5408#L157)
``` go
func (f Float64Data) SoftMax() ([]float64, error)
```
SoftMax returns the input values in the range of 0 to 1
with sum of all the probabilities being equal to one.




### <a name="Float64Data.StandardDeviation">func</a> (Float64Data) [StandardDeviation](/data.go?s=2026:2083#L56)
``` go
func (f Float64Data) StandardDeviation() (float64, error)
```
StandardDeviation the amount of variation in the dataset




### <a name="Float64Data.StandardDeviationPopulation">func</a> (Float64Data) [StandardDeviationPopulation](/data.go?s=2199:2266#L61)
``` go
func (f Float64Data) StandardDeviationPopulation() (float64, error)
```
StandardDeviationPopulation finds the amount of variation from the population




### <a name="Float64Data.StandardDeviationSample">func</a> (Float64Data) [StandardDeviationSample](/data.go?s=2382:2445#L66)
``` go
func (f Float64Data) StandardDeviationSample() (float64, error)
```
StandardDeviationSample finds the amount of variation from a sample




### <a name="Float64Data.Sum">func</a> (Float64Data) [Sum](/data.go?s=764:807#L25)
``` go
func (f Float64Data) Sum() (float64, error)
```
Sum returns the total of all the numbers in the data




### <a name="Float64Data.Swap">func</a> (Float64Data) [Swap](/data.go?s=425:460#L16)
``` go
func (f Float64Data) Swap(i, j int)
```
Swap switches out two numbers in slice




### <a name="Float64Data.Trimean">func</a> (Float64Data) [Trimean](/data.go?s=4059:4119#L116)
``` go
func (f Float64Data) Trimean(d Float64Data) (float64, error)
```
Trimean finds the average of the median and the midhinge




### <a name="Float64Data.Variance">func</a> (Float64Data) [Variance](/data.go?s=4350:4398#L126)
``` go
func (f Float64Data) Variance() (float64, error)
```
Variance the amount of variation in the dataset




## <a name="Outliers">type</a> [Outliers](/outlier.go?s=73:139#L4)
``` go
type Outliers struct {
    Mild    Float64Data
    Extreme Float64Data
}

```
Outliers holds mild and extreme outliers found in data







### <a name="QuartileOutliers">func</a> [QuartileOutliers](/outlier.go?s=197:255#L10)
``` go
func QuartileOutliers(input Float64Data) (Outliers, error)
```
QuartileOutliers finds the mild and extreme outliers





## <a name="Quartiles">type</a> [Quartiles](/quartile.go?s=75:136#L6)
``` go
type Quartiles struct {
    Q1 float64
    Q2 float64
    Q3 float64
}

```
Quartiles holds the three quartile points







### <a name="Quartile">func</a> [Quartile](/quartile.go?s=205:256#L13)
``` go
func Quartile(input Float64Data) (Quartiles, error)
```
Quartile returns the three quartile points from a slice of data





## <a name="Series">type</a> [Series](/regression.go?s=76:100#L6)
``` go
type Series []Coordinate
```
Series is a container for a series of data







### <a name="ExponentialRegression">func</a> [ExponentialRegression](/regression.go?s=1089:1157#L50)
``` go
func ExponentialRegression(s Series) (regressions Series, err error)
```
ExponentialRegression returns an exponential regression on data series


### <a name="LinearRegression">func</a> [LinearRegression](/regression.go?s=262:325#L14)
``` go
func LinearRegression(s Series) (regressions Series, err error)
```
LinearRegression finds the least squares linear regression on data series


### <a name="LogarithmicRegression">func</a> [LogarithmicRegression](/regression.go?s=1903:1971#L85)
``` go
func LogarithmicRegression(s Series) (regressions Series, err error)
```
LogarithmicRegression returns an logarithmic regression on data series









- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)
