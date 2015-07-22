// +build ignore

package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/vbatts/tar-split/archive/tar"
)

func main() {
	flag.Parse()
	log.SetOutput(os.Stderr)
	for _, arg := range flag.Args() {
		func() {
			// Open the tar archive
			fh, err := os.Open(arg)
			if err != nil {
				log.Fatal(err, arg)
			}
			defer fh.Close()

			output, err := os.Create(fmt.Sprintf("%s.out", arg))
			if err != nil {
				log.Fatal(err)
			}
			defer output.Close()
			log.Printf("writing %q to %q", fh.Name(), output.Name())

			fi, err := fh.Stat()
			if err != nil {
				log.Fatal(err, fh.Name())
			}
			size := fi.Size()
			var sum int64
			tr := tar.NewReader(fh)
			tr.RawAccounting = true
			for {
				hdr, err := tr.Next()
				if err != nil {
					if err != io.EOF {
						log.Println(err)
					}
					// even when an EOF is reached, there is often 1024 null bytes on
					// the end of an archive. Collect them too.
					post := tr.RawBytes()
					output.Write(post)
					sum += int64(len(post))

					fmt.Printf("EOF padding: %d\n", len(post))
					break
				}

				pre := tr.RawBytes()
				output.Write(pre)
				sum += int64(len(pre))

				var i int64
				if i, err = io.Copy(output, tr); err != nil {
					log.Println(err)
					break
				}
				sum += i

				fmt.Println(hdr.Name, "pre:", len(pre), "read:", i)
			}

			// it is allowable, and not uncommon that there is further padding on the
			// end of an archive, apart from the expected 1024 null bytes
			remainder, err := ioutil.ReadAll(fh)
			if err != nil && err != io.EOF {
				log.Fatal(err, fh.Name())
			}
			output.Write(remainder)
			sum += int64(len(remainder))
			fmt.Printf("Remainder: %d\n", len(remainder))

			if size != sum {
				fmt.Printf("Size: %d; Sum: %d; Diff: %d\n", size, sum, size-sum)
				fmt.Printf("Compare like `cmp -bl %s %s | less`\n", fh.Name(), output.Name())
			} else {
				fmt.Printf("Size: %d; Sum: %d\n", size, sum)
			}
		}()
	}
}
