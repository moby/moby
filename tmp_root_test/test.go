package main
import (
 "fmt"
 "os"
)
func main() {
 tmp := os.Args[1]
 root, err := os.OpenRoot(tmp)
 if err != nil { panic(err) }
 defer root.Close()
 for _, path := range []string{"foo", "foo/bar", "./foo", "./foo/bar", "foo\\bar", "./foo\\bar"} {
  err := root.Mkdir(path, 0o755)
  fmt.Printf("path=%q err=%v\n", path, err)
 }
}
