package metrics

func sumFloat64(vs ...float64) float64 {
	var sum float64
	for _, v := range vs {
		sum += v
	}

	return sum
}
