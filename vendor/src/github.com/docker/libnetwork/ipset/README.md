## Synopsis
ipset.go offers an layer to access ipset from within Docker. Ultimately, ipset.go should support all case operations that ipset allows. ipset.go should also validate all inputs that is given and should offer helper functions to automate certain frequent operations.

The aim of this project is to allow easy implementation of ipset related operations. 

This project is still work in progress, I very much appreciate feedback, so far this is just a simple prototype, there is some more work needed to be done in this.  

## Example usage

```go
//This is just for a test
package main
import (

    "fmt"
    "github.com/docker/libnetwork/ipset/ipset"
)

func main() {

    var err error
    var output string
    var options []string
    var typenet = []string {"net","net"}
    var createit =map[string] string{
        "timeout"  : "30",
        "counters" : "",
    }
    
    output, err = ipset.Create("myset", "hash", typenet, createit)
    fmt.Println("Create:", output ,err)
    
    output, err = ipset.Save("myset", options)
    fmt.Println("Save:", output ,err)

    output, err = ipset.List("myset", options)
    fmt.Println("List:", output ,err)

    output, err = ipset.Destroy("myset", options)
    fmt.Println("Destroy:", output ,err)

    output, err = ipset.Flush("myset", options)
    fmt.Println("Flush:", output ,err)
}
```



