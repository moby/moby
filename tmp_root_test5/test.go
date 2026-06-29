package main
import (
 "fmt"
 "os"
)
func main() {
 root, err := os.OpenRoot(os.Args[1])
 if err != nil { panic(err) }
 defer root.Close()
 for _, path := range []string{"foo",".\\foo","foo\\bar",".\\foo\\bar"} {
  fmt.Printf("test path=%q\n", path)
  if fi, err := root.Stat(path); err != nil { fmt.Printf(" Stat err=%v\n", err) } else { fmt.Printf(" Stat exists=%v dir=%v\n", true, fi.IsDir()) }
  err = root.Mkdir(path, 0o755)
  fmt.Printf(" Mkdir err=%v\n", err)
  if fi, err := root.Stat(path); err != nil { fmt.Printf(" Stat2 err=%v\n", err) } else { fmt.Printf(" Stat2 exists=%v dir=%v\n", true, fi.IsDir()) }
 }
}
