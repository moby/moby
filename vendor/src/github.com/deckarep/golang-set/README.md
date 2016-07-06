[![Build Status](https://travis-ci.org/deckarep/golang-set.png?branch=master)](https://travis-ci.org/deckarep/golang-set)
[![GoDoc](https://godoc.org/github.com/deckarep/golang-set?status.png)](http://godoc.org/github.com/deckarep/golang-set)

## golang-set


The missing set collection for the Go language.  Until Go has sets built-in...use this.

Coming from Python one of the things I miss is the superbly wonderful set collection.  This is my attempt to mimic the primary features of the set from Python.
You can of course argue that there is no need for a set in Go, otherwise the creators would have added one to the standard library.  To those I say simply ignore this repository
and carry-on and to the rest that find this useful please contribute in helping me make it better by:

* Helping to make more idiomatic improvements to the code.
* Helping to increase the performance of it. ~~(So far, no attempt has been made, but since it uses a map internally, I expect it to be mostly performant.)~~
* Helping to make the unit-tests more robust and kick-ass.
* Helping to fill in the [documentation.](http://godoc.org/github.com/deckarep/golang-set)
* Simply offering feedback and suggestions.  (Positive, constructive feedback is appreciated.)

I have to give some credit for helping seed the idea with this post on [stackoverflow.](http://programmers.stackexchange.com/questions/177428/sets-data-structure-in-golang)

*Update* - as of 3/9/2014, you can use a compile-time generic version of this package in the [gen](http://clipperhouse.github.io/gen/) framework.  This framework allows you to use the golang-set in a completely generic and type-safe way by allowing you to generate a supporting .go file based on your custom types.

## Features (as of 9/22/2014)

* a CartesianProduct() method has been added with unit-tests: [Read more about the cartesian product](http://en.wikipedia.org/wiki/Cartesian_product)

## Features (as of 9/15/2014)

* a PowerSet() method has been added with unit-tests: [Read more about the Power set](http://en.wikipedia.org/wiki/Power_set)

## Features (as of 4/22/2014)

* One common interface to both implementations
* Two set implementations to choose from
  * a thread-safe implementation designed for concurrent use
  * a non-thread-safe implementation designed for performance
* 75 benchmarks for both implementations
* 35 unit tests for both implementations
* 14 concurrent tests for the thread-safe implementation



Please see the unit test file for additional usage examples.  The Python set documentation will also do a better job than I can of explaining how a set typically [works.](http://docs.python.org/2/library/sets.html)    Please keep in mind
however that the Python set is a built-in type and supports additional features and syntax that make it awesome.

## Examples but not exhaustive:

```go
requiredClasses := mapset.NewSet()
requiredClasses.Add("Cooking")
requiredClasses.Add("English")
requiredClasses.Add("Math")
requiredClasses.Add("Biology")

scienceSlice := []interface{}{"Biology", "Chemistry"}
scienceClasses := mapset.NewSetFromSlice(scienceSlice)

electiveClasses := mapset.NewSet()
electiveClasses.Add("Welding")
electiveClasses.Add("Music")
electiveClasses.Add("Automotive")

bonusClasses := mapset.NewSet()
bonusClasses.Add("Go Programming")
bonusClasses.Add("Python Programming")

//Show me all the available classes I can take
allClasses := requiredClasses.Union(scienceClasses).Union(electiveClasses).Union(bonusClasses)
fmt.Println(allClasses) //Set{Cooking, English, Math, Chemistry, Welding, Biology, Music, Automotive, Go Programming, Python Programming}


//Is cooking considered a science class?
fmt.Println(scienceClasses.Contains("Cooking")) //false

//Show me all classes that are not science classes, since I hate science.
fmt.Println(allClasses.Difference(scienceClasses)) //Set{Music, Automotive, Go Programming, Python Programming, Cooking, English, Math, Welding}

//Which science classes are also required classes?
fmt.Println(scienceClasses.Intersect(requiredClasses)) //Set{Biology}

//How many bonus classes do you offer?
fmt.Println(bonusClasses.Cardinality()) //2

//Do you have the following classes? Welding, Automotive and English?
fmt.Println(allClasses.IsSuperset(mapset.NewSetFromSlice([]interface{}{"Welding", "Automotive", "English"}))) //true
```

Thanks!

-Ralph

[![Bitdeli Badge](https://d2weczhvl823v0.cloudfront.net/deckarep/golang-set/trend.png)](https://bitdeli.com/free "Bitdeli Badge")

[![Analytics](https://ga-beacon.appspot.com/UA-42584447-2/deckarep/golang-set)](https://github.com/igrigorik/ga-beacon)
