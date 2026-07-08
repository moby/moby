package main
import (
 "fmt"
 "os"
 "path/filepath"
 "strings"
)
func scopedMkdirAll(root *os.Root, path string) error {
 dir := ""
 for _, part := range strings.Split(filepath.ToSlash(path), "/") {
  if part == "" || part == "." {
   continue
  }
  dir = filepath.Join(dir, part)
  if fi, err := root.Stat(dir); err == nil {
   if !fi.IsDir() {
    return err
   }
   continue
  }
  if err := root.Mkdir(dir, 0o755); err != nil {
   return err
  }
 }
 return nil
}
func main() {
 root, err := os.OpenRoot(os.Args[1])
 if err != nil { panic(err) }
 defer root.Close()
 for _, path := range []string{"foo/bar", ".///foo/bar", "./foo/bar", "foo\\bar", "./foo\\bar"} {
  fmt.Printf("path=%q\n", path)
  err := scopedMkdirAll(root, path)
  fmt.Printf(" err=%v\n", err)
 }
}
