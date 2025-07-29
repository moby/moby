/*
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

MIT License Copyright (c) 2014-2020 Montana Flynn (https://montanaflynn.com)
*/
package stats
