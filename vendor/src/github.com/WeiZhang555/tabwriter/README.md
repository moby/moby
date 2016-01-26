
# tabwriter

This is a modified version of Golang text/tabwriter for better processing East Asian Characters. The original Golang tabwriter assumes all unicode chars have same width which is often not true, especially for CJK (Chinese, Japanise and Korean) ideograph words. 

This modified tabwriter will calculate real display width based on unicode ranges, give CJK words two columes for display.

## Getting Started

```
go get github.com/WeiZhang555/tabwriter
```

## Synopsis

This is a very simple example:

```go
package main

import (
	"fmt"
	"io"

	"github.com/WeiZhang555/tabwriter"
)

type myWriter struct {
	io.Writer
}

func (w myWriter) Write(p []byte) (n int, err error) {
	fmt.Printf("%s", string(p))
	return len(p), nil
}

func main() {
	myw := myWriter{}

	tw := tabwriter.NewWriter(myw, 10, 1, 3, ' ', 0)
	str := "hello\tthis\tis\ta\ttest\tfrom\twei\n"
	str1 := "你好\thello\t世界\tworld\t。\t再见\t：）\n"

	fmt.Fprintf(tw, str)
	fmt.Fprintf(tw, str1)
	tw.Flush()
}
```

The output will looks like this:

```
hello     this      is        a         test      from      wei
你好      hello     世界      world     。        再见      ：）
```

While with Golang's official `text/tabwriter`, it will be like this:

```
hello     this      is        a         test      from      wei
你好        hello     世界        world     。         再见        ：）
```

This tabwriter has exactly the same function list with Golang text/tabwriter, and almost the same implementations with original tabwriter, except that display width of CJK words is 2 but not 1.

## Contributing

Any bug, issue or contributing PR is welcome! Feel free to talk to me if you have any problem using it!
