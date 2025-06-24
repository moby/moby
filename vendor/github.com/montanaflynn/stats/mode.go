package stats

// Mode gets the mode [most frequent value(s)] of a slice of float64s
func Mode(input Float64Data) (mode []float64, err error) {
	// Return the input if there's only one number
	l := input.Len()
	if l == 1 {
		return input, nil
	} else if l == 0 {
		return nil, EmptyInputErr
	}

	c := sortedCopyDif(input)
	// Traverse sorted array,
	// tracking the longest repeating sequence
	mode = make([]float64, 5)
	cnt, maxCnt := 1, 1
	for i := 1; i < l; i++ {
		switch {
		case c[i] == c[i-1]:
			cnt++
		case cnt == maxCnt && maxCnt != 1:
			mode = append(mode, c[i-1])
			cnt = 1
		case cnt > maxCnt:
			mode = append(mode[:0], c[i-1])
			maxCnt, cnt = cnt, 1
		default:
			cnt = 1
		}
	}
	switch {
	case cnt == maxCnt:
		mode = append(mode, c[l-1])
	case cnt > maxCnt:
		mode = append(mode[:0], c[l-1])
		maxCnt = cnt
	}

	// Since length must be greater than 1,
	// check for slices of distinct values
	if maxCnt == 1 || len(mode)*maxCnt == l && maxCnt != l {
		return Float64Data{}, nil
	}

	return mode, nil
}
