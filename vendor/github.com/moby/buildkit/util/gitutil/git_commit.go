package gitutil

func IsCommitSHA(str string) bool {
	if l := len(str); l != 40 && l != 64 {
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
