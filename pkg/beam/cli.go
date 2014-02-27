package beam

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

// Cli scans `stdin` for user input, sending each line in a separate message,
// along with a return stream which will print anything that is sent to it
// on `stdout`, with annotated with a prefix to distinguish streams.
// As a special case, if the return stream receives a message
// with `Data=[]byte("stderr")` and a valid stream, that stream will be
// copied to `stderr`.
func Cli(stdin io.ReadCloser, stdout io.Writer, stderr io.Writer) Stream {
	inside, outside := Pipe()
	input := bufio.NewScanner(stdin)
	var tasks sync.WaitGroup
	go func() {
		defer tasks.Wait()
		defer inside.Close()
		for id := 0; ; id++ {
			if !input.Scan() {
				return
			}
			local, remote := Pipe()
			msg := Message{
				Data:   input.Bytes(),
				Stream: remote,
			}
			if len(msg.Data) == 0 && input.Err() == nil {
				continue
			}
			tasks.Add(1)
			go func(id int) {
				fmt.Printf("New task with id=%d\n", id)
				defer tasks.Done()
				defer local.Close()
				for i := 0; true; i++ {
					m, err := local.Receive()
					if err != nil {
						return
					}
					fmt.Fprintf(stdout, "[%d] %s\n", id, m.Data)
					if m.Stream == nil {
						continue
					}
					name := string(m.Data)
					prefix := fmt.Sprintf("[%d] [%s] ", id, name)
					var output io.Writer
					if name == "stderr" {
						output = prefixer(prefix, stderr)
					} else {
						output = prefixer(prefix, stdout)
					}
					tasks.Add(1)
					go func() {
						io.Copy(output, NewReader(m.Stream))
						fmt.Fprintf(output, "<EOF>\n")
						tasks.Done()
					}()
				}
			}(id)
			if err := inside.Send(msg); err != nil {
				return
			}
		}
	}()
	return outside
}

func prefixer(prefix string, output io.Writer) io.Writer {
	r, w := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := prefix + scanner.Text()
			if len(line) == 0 || line[len(line)-1] != '\n' {
				line = line + "\n"
			}
			fmt.Fprintf(output, "%s", line)
		}
	}()
	return w
}
