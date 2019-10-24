package types

import "os"

func (s Stat) IsDir() bool {
	return os.FileMode(s.Mode).IsDir()
}
