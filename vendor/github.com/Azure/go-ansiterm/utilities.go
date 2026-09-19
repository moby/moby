package ansiterm

func sliceContains(bytes []byte, b byte) bool {
	for _, v := range bytes {
		if v == b {
			return true
		}
	}

	return false
}
