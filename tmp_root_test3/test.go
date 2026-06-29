package main
import (
 "fmt"
 "os"
)
func main() {
 root, err := os.OpenRoot(os.Args[1])
 if err != nil { panic(err) }
 defer root.Close()
 for _, path := range []string{"tmp", "./tmp", ".\\tmp", "tmp\\sub", "./tmp\\sub"} {
  err := root.Mkdir(path, 0o755)
  fmt.Printf("path=%q err=%v\n", path, err)
 }
}
