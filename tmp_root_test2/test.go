package main
import (
 "fmt"
 "path/filepath"
)
func main() {
 fmt.Println(filepath.Join(`C:\base`, `\foo`))
 fmt.Println(filepath.Join(`C:\base`, `/foo`))
 fmt.Println(filepath.Join(`C:\base`, `foo`))
 fmt.Println(filepath.Join(`C:\base`, `//foo`))
 fmt.Println(filepath.Join(`C:\base`, `C:\foo`))
}
