package stats

import (
	"math"
	"math/rand"
	"strings"
	"time"
)

// NormPpfRvs generates random variates using the Point Percentile Function.
// For more information please visit: https://demonstrations.wolfram.com/TheMethodOfInverseTransforms/
func NormPpfRvs(loc float64, scale float64, size int) []float64 {
	rand.Seed(time.Now().UnixNano())
	var toReturn []float64
	for i := 0; i < size; i++ {
		toReturn = append(toReturn, NormPpf(rand.Float64(), loc, scale))
	}
	return toReturn
}

// NormBoxMullerRvs generates random variates using the Box–Muller transform.
// For more information please visit: http://mathworld.wolfram.com/Box-MullerTransformation.html
func NormBoxMullerRvs(loc float64, scale float64, size int) []float64 {
	rand.Seed(time.Now().UnixNano())
	var toReturn []float64
	for i := 0; i < int(float64(size/2)+float64(size%2)); i++ {
		// u1 and u2 are uniformly distributed random numbers between 0 and 1.
		u1 := rand.Float64()
		u2 := rand.Float64()
		// x1 and x2 are normally distributed random numbers.
		x1 := loc + (scale * (math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)))
		toReturn = append(toReturn, x1)
		if (i+1)*2 <= size {
			x2 := loc + (scale * (math.Sqrt(-2*math.Log(u1)) * math.Sin(2*math.Pi*u2)))
			toReturn = append(toReturn, x2)
		}
	}
	return toReturn
}

// NormPdf is the probability density function.
func NormPdf(x float64, loc float64, scale float64) float64 {
	return (math.Pow(math.E, -(math.Pow(x-loc, 2))/(2*math.Pow(scale, 2)))) / (scale * math.Sqrt(2*math.Pi))
}

// NormLogPdf is the log of the probability density function.
func NormLogPdf(x float64, loc float64, scale float64) float64 {
	return math.Log((math.Pow(math.E, -(math.Pow(x-loc, 2))/(2*math.Pow(scale, 2)))) / (scale * math.Sqrt(2*math.Pi)))
}

// NormCdf is the cumulative distribution function.
func NormCdf(x float64, loc float64, scale float64) float64 {
	return 0.5 * (1 + math.Erf((x-loc)/(scale*math.Sqrt(2))))
}

// NormLogCdf is the log of the cumulative distribution function.
func NormLogCdf(x float64, loc float64, scale float64) float64 {
	return math.Log(0.5 * (1 + math.Erf((x-loc)/(scale*math.Sqrt(2)))))
}

// NormSf is the survival function (also defined as 1 - cdf, but sf is sometimes more accurate).
func NormSf(x float64, loc float64, scale float64) float64 {
	return 1 - 0.5*(1+math.Erf((x-loc)/(scale*math.Sqrt(2))))
}

// NormLogSf is the log of the survival function.
func NormLogSf(x float64, loc float64, scale float64) float64 {
	return math.Log(1 - 0.5*(1+math.Erf((x-loc)/(scale*math.Sqrt(2)))))
}

// NormPpf is the point percentile function.
// This is based on Peter John Acklam's inverse normal CDF.
// algorithm: http://home.online.no/~pjacklam/notes/invnorm/ (no longer visible).
// For more information please visit: https://stackedboxes.org/2017/05/01/acklams-normal-quantile-function/
func NormPpf(p float64, loc float64, scale float64) (x float64) {
	const (
		a1 = -3.969683028665376e+01
		a2 = 2.209460984245205e+02
		a3 = -2.759285104469687e+02
		a4 = 1.383577518672690e+02
		a5 = -3.066479806614716e+01
		a6 = 2.506628277459239e+00

		b1 = -5.447609879822406e+01
		b2 = 1.615858368580409e+02
		b3 = -1.556989798598866e+02
		b4 = 6.680131188771972e+01
		b5 = -1.328068155288572e+01

		c1 = -7.784894002430293e-03
		c2 = -3.223964580411365e-01
		c3 = -2.400758277161838e+00
		c4 = -2.549732539343734e+00
		c5 = 4.374664141464968e+00
		c6 = 2.938163982698783e+00

		d1 = 7.784695709041462e-03
		d2 = 3.224671290700398e-01
		d3 = 2.445134137142996e+00
		d4 = 3.754408661907416e+00

		plow  = 0.02425
		phigh = 1 - plow
	)

	if p < 0 || p > 1 {
		return math.NaN()
	} else if p == 0 {
		return -math.Inf(0)
	} else if p == 1 {
		return math.Inf(0)
	}

	if p < plow {
		q := math.Sqrt(-2 * math.Log(p))
		x = (((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	} else if phigh < p {
		q := math.Sqrt(-2 * math.Log(1-p))
		x = -(((((c1*q+c2)*q+c3)*q+c4)*q+c5)*q + c6) /
			((((d1*q+d2)*q+d3)*q+d4)*q + 1)
	} else {
		q := p - 0.5
		r := q * q
		x = (((((a1*r+a2)*r+a3)*r+a4)*r+a5)*r + a6) * q /
			(((((b1*r+b2)*r+b3)*r+b4)*r+b5)*r + 1)
	}

	e := 0.5*math.Erfc(-x/math.Sqrt2) - p
	u := e * math.Sqrt(2*math.Pi) * math.Exp(x*x/2)
	x = x - u/(1+x*u/2)

	return x*scale + loc
}

// NormIsf is the inverse survival function (inverse of sf).
func NormIsf(p float64, loc float64, scale float64) (x float64) {
	if -NormPpf(p, loc, scale) == 0 {
		return 0
	}
	return -NormPpf(p, loc, scale)
}

// NormMoment approximates the non-central (raw) moment of order n.
// For more information please visit: https://math.stackexchange.com/questions/1945448/methods-for-finding-raw-moments-of-the-normal-distribution
func NormMoment(n int, loc float64, scale float64) float64 {
	toReturn := 0.0
	for i := 0; i < n+1; i++ {
		if (n-i)%2 == 0 {
			toReturn += float64(Ncr(n, i)) * (math.Pow(loc, float64(i))) * (math.Pow(scale, float64(n-i))) *
				(float64(factorial(n-i)) / ((math.Pow(2.0, float64((n-i)/2))) *
					float64(factorial((n-i)/2))))
		}
	}
	return toReturn
}

// NormStats returns the mean, variance, skew, and/or kurtosis.
// Mean(‘m’), variance(‘v’), skew(‘s’), and/or kurtosis(‘k’).
// Takes string containing any of 'mvsk'.
// Returns array of m v s k in that order.
func NormStats(loc float64, scale float64, moments string) []float64 {
	var toReturn []float64
	if strings.ContainsAny(moments, "m") {
		toReturn = append(toReturn, loc)
	}
	if strings.ContainsAny(moments, "v") {
		toReturn = append(toReturn, math.Pow(scale, 2))
	}
	if strings.ContainsAny(moments, "s") {
		toReturn = append(toReturn, 0.0)
	}
	if strings.ContainsAny(moments, "k") {
		toReturn = append(toReturn, 0.0)
	}
	return toReturn
}

// NormEntropy is the differential entropy of the RV.
func NormEntropy(loc float64, scale float64) float64 {
	return math.Log(scale * math.Sqrt(2*math.Pi*math.E))
}

// NormFit returns the maximum likelihood estimators for the Normal Distribution.
// Takes array of float64 values.
// Returns array of Mean followed by Standard Deviation.
func NormFit(data []float64) [2]float64 {
	sum := 0.00
	for i := 0; i < len(data); i++ {
		sum += data[i]
	}
	mean := sum / float64(len(data))
	stdNumerator := 0.00
	for i := 0; i < len(data); i++ {
		stdNumerator += math.Pow(data[i]-mean, 2)
	}
	return [2]float64{mean, math.Sqrt((stdNumerator) / (float64(len(data))))}
}

// NormMedian is the median of the distribution.
func NormMedian(loc float64, scale float64) float64 {
	return loc
}

// NormMean is the mean/expected value of the distribution.
func NormMean(loc float64, scale float64) float64 {
	return loc
}

// NormVar is the variance of the distribution.
func NormVar(loc float64, scale float64) float64 {
	return math.Pow(scale, 2)
}

// NormStd is the standard deviation of the distribution.
func NormStd(loc float64, scale float64) float64 {
	return scale
}

// NormInterval finds endpoints of the range that contains alpha percent of the distribution.
func NormInterval(alpha float64, loc float64, scale float64) [2]float64 {
	q1 := (1.0 - alpha) / 2
	q2 := (1.0 + alpha) / 2
	a := NormPpf(q1, loc, scale)
	b := NormPpf(q2, loc, scale)
	return [2]float64{a, b}
}

// factorial is the naive factorial algorithm.
func factorial(x int) int {
	if x == 0 {
		return 1
	}
	return x * factorial(x-1)
}

// Ncr is an N choose R algorithm.
// Aaron Cannon's algorithm.
func Ncr(n, r int) int {
	if n <= 1 || r == 0 || n == r {
		return 1
	}
	if newR := n - r; newR < r {
		r = newR
	}
	if r == 1 {
		return n
	}
	ret := int(n - r + 1)
	for i, j := ret+1, int(2); j <= r; i, j = i+1, j+1 {
		ret = ret * i / j
	}
	return ret
}
