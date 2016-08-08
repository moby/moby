package splice

import (
	"io"
	"os"
)

func SpliceCopy(dst *os.File, src *os.File, p *Pair) (int64, error) {
	total := int64(0)

	for {
		n, err := p.LoadFrom(src.Fd(), p.size)
		if err != nil {
			return total, err
		}
		if n == 0 {
			break
		}
		m, err := p.WriteTo(dst.Fd(), n)
		total += int64(m)
		if err != nil {
			return total, err
		}
		if m < n {
			return total, err
		}
		if int(n) < p.size {
			break
		}
	}

	return total, nil
}

// Argument ordering follows io.Copy.
func CopyFile(dstName string, srcName string, mode int) error {
	src, err := os.Open(srcName)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer dst.Close()

	return CopyFds(dst, src)
}

func CopyFds(dst *os.File, src *os.File) (err error) {
	p, err := splicePool.get()
	if p != nil {
		p.Grow(256 * 1024)
		_, err := SpliceCopy(dst, src, p)
		splicePool.done(p)
		return err
	} else {
		_, err = io.Copy(dst, src)
	}
	if err == io.EOF {
		err = nil
	}
	return err
}
