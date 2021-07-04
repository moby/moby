package caller

import (
	"runtime"
	"strings"
)

func callerInfo(i int) string {
	ptr, _, _, ok := runtime.Caller(i)
	fName := "unknown"
	if ok {
		f := runtime.FuncForPC(ptr)
		if f != nil {
			// f.Name() is like: github.com/docker/libnetwork/caller.MethodName
			tmp := strings.Split(f.Name(), ".")
			if len(tmp) > 0 {
				fName = tmp[len(tmp)-1]
			}
		}
	}

	return fName
}

// Name returns the name of the function at the specified level
// level == 0 means current method name
func Name(level int) string {
	return callerInfo(2 + level)
}
