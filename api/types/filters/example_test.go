package filters // import "github.com/docker/docker/api/types/filters"
import "fmt"

func ExampleArgs_MatchKVList() {
	args := NewArgs(
		Arg("label", "image=foo"),
		Arg("label", "state=running"))

	// returns true because there are no values for bogus
	b := args.MatchKVList("bogus", nil)
	fmt.Println(b)

	// returns false because there are no sources
	b = args.MatchKVList("label", nil)
	fmt.Println(b)

	// returns true because all sources are matched
	b = args.MatchKVList("label", map[string]string{
		"image": "foo",
		"state": "running",
	})
	fmt.Println(b)

	// returns false because the values do not match
	b = args.MatchKVList("label", map[string]string{
		"image": "other",
	})
	fmt.Println(b)

	// Output:
	// true
	// false
	// true
	// false
}
