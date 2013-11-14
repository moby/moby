package namesgenerator

import (
	"fmt"
	"math/rand"
	"time"
)

type NameChecker interface {
	Exists(name string) bool
}

var (
	colors  = [...]string{"white", "silver", "gray", "black", "blue", "green", "cyan", "yellow", "gold", "orange", "brown", "red", "violet", "pink", "magenta", "purple", "maroon", "crimson", "plum", "fuchsia", "lavender", "slate", "navy", "azure", "aqua", "olive", "teal", "lime", "beige", "tan", "sienna"}
	animals = [...]string{"ant", "bear", "bird", "cat", "chicken", "cow", "deer", "dog", "donkey", "duck", "fish", "fox", "frog", "horse", "kangaroo", "koala", "lemur", "lion", "lizard", "monkey", "octopus", "pig", "shark", "sheep", "sloth", "spider", "squirrel", "tiger", "toad", "weasel", "whale", "wolf"}
)

func GenerateRandomName(checker NameChecker) (string, error) {
	retry := 5
	rand.Seed(time.Now().UnixNano())
	name := fmt.Sprintf("%s_%s", colors[rand.Intn(len(colors))], animals[rand.Intn(len(animals))])
	for checker != nil && checker.Exists(name) && retry > 0 {
		name = fmt.Sprintf("%s%d", name, rand.Intn(10))
		retry = retry - 1
	}
	if retry == 0 {
		return name, fmt.Errorf("Error generating random name")
	}
	return name, nil
}
