package gitutil

func IsCommitSHA(str string) bool {
	if len(str) != 40 {
		return false
	}

	for _, ch := range str {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch >= 'a' && ch <= 'f' {
			continue
		}
		return false
	}

	return true
}
