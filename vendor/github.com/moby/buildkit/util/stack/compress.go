package stack

import (
	"slices"
)

func compressStacks(st []*Stack) []*Stack {
	if len(st) == 0 {
		return nil
	}

	slices.SortFunc(st, func(a, b *Stack) int {
		return len(b.Frames) - len(a.Frames)
	})

	out := []*Stack{st[0]}

loop0:
	for _, st := range st[1:] {
		maxIdx := -1
		for _, prev := range out {
			idx := subFrames(st.Frames, prev.Frames)
			if idx == -1 {
				continue
			}
			// full match, potentially skip all
			if idx == len(st.Frames)-1 {
				if st.Pid == prev.Pid && st.Version == prev.Version && slices.Compare(st.Cmdline, st.Cmdline) == 0 {
					continue loop0
				}
			}
			if idx > maxIdx {
				maxIdx = idx
			}
		}

		if maxIdx > 0 {
			st.Frames = st.Frames[:len(st.Frames)-maxIdx]
		}
		out = append(out, st)
	}

	return out
}

func subFrames(a, b []*Frame) int {
	idx := -1
	i := len(a) - 1
	j := len(b) - 1
	for i >= 0 {
		if j < 0 {
			break
		}
		if a[i].Equal(b[j]) {
			idx++
			i--
			j--
		} else {
			break
		}
	}
	return idx
}

func (a *Frame) Equal(b *Frame) bool {
	return a.File == b.File && a.Line == b.Line && a.Name == b.Name
}
