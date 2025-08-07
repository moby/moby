package stats

// import "math"
//
// // WilcoxonRankSum tests the null hypothesis that two sets
// // of data are drawn from the same distribution. It does
// // not handle ties between measurements in x and y.
// //
// // Parameters:
// //    data1 Float64Data: First set of data points.
// //    data2 Float64Data: Second set of data points.
// //    Length of both data samples must be equal.
// //
// // Return:
// //    statistic float64: The test statistic under the
// //                       large-sample approximation that the
// //                       rank sum statistic is normally distributed.
// //    pvalue float64: The two-sided p-value of the test
// //    err error: Any error from the input data parameters
// //
// // https://en.wikipedia.org/wiki/Wilcoxon_rank-sum_test
// func WilcoxonRankSum(data1, data2 Float64Data) (float64, float64, error) {
//
// 	l1 := data1.Len()
// 	l2 := data2.Len()
//
// 	if l1 == 0 || l2 == 0 {
// 		return math.NaN(), math.NaN(), EmptyInputErr
// 	}
//
// 	if l1 != l2 {
// 		return math.NaN(), math.NaN(), SizeErr
// 	}
//
// 	alldata := Float64Data{}
// 	alldata = append(alldata, data1...)
// 	alldata = append(alldata, data2...)
//
// 	// ranked :=
//
// 	return 0.0, 0.0, nil
// }
//
// //     x, y = map(np.asarray, (x, y))
// //     n1 = len(x)
// //     n2 = len(y)
// //     alldata = np.concatenate((x, y))
// //     ranked = rankdata(alldata)
// //     x = ranked[:n1]
// //     s = np.sum(x, axis=0)
// //     expected = n1 * (n1+n2+1) / 2.0
// //     z = (s - expected) / np.sqrt(n1*n2*(n1+n2+1)/12.0)
// //     prob = 2 * distributions.norm.sf(abs(z))
// //
// //     return RanksumsResult(z, prob)
//
// // def rankdata(a, method='average'):
// //     """
// //     Assign ranks to data, dealing with ties appropriately.
// //     Ranks begin at 1.  The `method` argument controls how ranks are assigned
// //     to equal values.  See [1]_ for further discussion of ranking methods.
// //     Parameters
// //     ----------
// //     a : array_like
// //         The array of values to be ranked.  The array is first flattened.
// //     method : str, optional
// //         The method used to assign ranks to tied elements.
// //         The options are 'average', 'min', 'max', 'dense' and 'ordinal'.
// //         'average':
// //             The average of the ranks that would have been assigned to
// //             all the tied values is assigned to each value.
// //         'min':
// //             The minimum of the ranks that would have been assigned to all
// //             the tied values is assigned to each value.  (This is also
// //             referred to as "competition" ranking.)
// //         'max':
// //             The maximum of the ranks that would have been assigned to all
// //             the tied values is assigned to each value.
// //         'dense':
// //             Like 'min', but the rank of the next highest element is assigned
// //             the rank immediately after those assigned to the tied elements.
// //         'ordinal':
// //             All values are given a distinct rank, corresponding to the order
// //             that the values occur in `a`.
// //         The default is 'average'.
// //     Returns
// //     -------
// //     ranks : ndarray
// //          An array of length equal to the size of `a`, containing rank
// //          scores.
// //     References
// //     ----------
// //     .. [1] "Ranking", https://en.wikipedia.org/wiki/Ranking
// //     Examples
// //     --------
// //     >>> from scipy.stats import rankdata
// //     >>> rankdata([0, 2, 3, 2])
// //     array([ 1. ,  2.5,  4. ,  2.5])
// //     """
// //
// //     arr = np.ravel(np.asarray(a))
// //     algo = 'quicksort'
// //     sorter = np.argsort(arr, kind=algo)
// //
// //     inv = np.empty(sorter.size, dtype=np.intp)
// //     inv[sorter] = np.arange(sorter.size, dtype=np.intp)
// //
// //
// //     arr = arr[sorter]
// //     obs = np.r_[True, arr[1:] != arr[:-1]]
// //     dense = obs.cumsum()[inv]
// //
// //
// //     # cumulative counts of each unique value
// //     count = np.r_[np.nonzero(obs)[0], len(obs)]
// //
// //     # average method
// //     return .5 * (count[dense] + count[dense - 1] + 1)
//
// type rankable interface {
// 	Len() int
// 	RankEqual(int, int) bool
// }
//
// func StandardRank(d rankable) []float64 {
// 	r := make([]float64, d.Len())
// 	var k int
// 	for i := range r {
// 		if i == 0 || !d.RankEqual(i, i-1) {
// 			k = i + 1
// 		}
// 		r[i] = float64(k)
// 	}
// 	return r
// }
//
// func ModifiedRank(d rankable) []float64 {
// 	r := make([]float64, d.Len())
// 	for i := range r {
// 		k := i + 1
// 		for j := i + 1; j < len(r) && d.RankEqual(i, j); j++ {
// 			k = j + 1
// 		}
// 		r[i] = float64(k)
// 	}
// 	return r
// }
//
// func DenseRank(d rankable) []float64 {
// 	r := make([]float64, d.Len())
// 	var k int
// 	for i := range r {
// 		if i == 0 || !d.RankEqual(i, i-1) {
// 			k++
// 		}
// 		r[i] = float64(k)
// 	}
// 	return r
// }
//
// func OrdinalRank(d rankable) []float64 {
// 	r := make([]float64, d.Len())
// 	for i := range r {
// 		r[i] = float64(i + 1)
// 	}
// 	return r
// }
//
// func FractionalRank(d rankable) []float64 {
// 	r := make([]float64, d.Len())
// 	for i := 0; i < len(r); {
// 		var j int
// 		f := float64(i + 1)
// 		for j = i + 1; j < len(r) && d.RankEqual(i, j); j++ {
// 			f += float64(j + 1)
// 		}
// 		f /= float64(j - i)
// 		for ; i < j; i++ {
// 			r[i] = f
// 		}
// 	}
// 	return r
// }
