// Package namesgenerator generates random names.
package namesgenerator

import (
	"math/rand"
	"strconv"
)

// For a long time, this package provided a lot of joy within the project, but
// at some point the conflicts of opinion became greater than the added joy.
//
// See also https://github.com/moby/moby/pull/43210#issuecomment-1029934277
//
// These word lists are not manually curated. Each word list is the complete
// contents of a rigorously defined, closed (unchanging) set of words. Additions
// or removals of individual words from a list will therefore not be accepted.
var (
	// An arbitrary, inexhaustive list of adjectives.
	// Grandfathered in: the contents of this list are frozen.
	//
	// TODO: replace with a word list that follows the guidelines.
	adjective = [...]string{
		"admiring",
		"adoring",
		"affectionate",
		"agitated",
		"amazing",
		"angry",
		"awesome",
		"beautiful",
		"blissful",
		"bold",
		"boring",
		"brave",
		"busy",
		"charming",
		"clever",
		"compassionate",
		"competent",
		"condescending",
		"confident",
		"cool",
		"cranky",
		"crazy",
		"dazzling",
		"determined",
		"distracted",
		"dreamy",
		"eager",
		"ecstatic",
		"elastic",
		"elated",
		"elegant",
		"eloquent",
		"epic",
		"exciting",
		"fervent",
		"festive",
		"flamboyant",
		"focused",
		"friendly",
		"frosty",
		"funny",
		"gallant",
		"gifted",
		"goofy",
		"gracious",
		"great",
		"happy",
		"hardcore",
		"heuristic",
		"hopeful",
		"hungry",
		"infallible",
		"inspiring",
		"intelligent",
		"interesting",
		"jolly",
		"jovial",
		"keen",
		"kind",
		"laughing",
		"loving",
		"lucid",
		"magical",
		"modest",
		"musing",
		"mystifying",
		"naughty",
		"nervous",
		"nice",
		"nifty",
		"nostalgic",
		"objective",
		"optimistic",
		"peaceful",
		"pedantic",
		"pensive",
		"practical",
		"priceless",
		"quirky",
		"quizzical",
		"recursing",
		"relaxed",
		"reverent",
		"romantic",
		"sad",
		"serene",
		"sharp",
		"silly",
		"sleepy",
		"stoic",
		"strange",
		"stupefied",
		"suspicious",
		"sweet",
		"tender",
		"thirsty",
		"trusting",
		"unruffled",
		"upbeat",
		"vibrant",
		"vigilant",
		"vigorous",
		"wizardly",
		"wonderful",
		"xenodochial",
		"youthful",
		"zealous",
		"zen",
	}

	// Isaac Newton's seven colours of the rainbow.
	// https://en.wikipedia.org/w/index.php?title=ROYGBIV&oldid=1334935060
	colour = [...]string{
		"red",
		"orange",
		"yellow",
		"green",
		"blue",
		"indigo",
		"violet",
	}

	// IUPAC names of the elements of the periodic table which have
	// primordial nuclides.
	// https://en.wikipedia.org/w/index.php?title=List_of_elements_by_stability_of_isotopes&oldid=1333877059
	element = [...]string{
		"hydrogen",
		"helium",
		"lithium",
		"beryllium",
		"boron",
		"carbon",
		"nitrogen",
		"oxygen",
		"fluorine",
		"neon",
		"sodium",
		"magnesium",
		"aluminium", // IUPAC spelling, not "aluminum"
		"silicon",
		"phosphorus",
		"sulfur",
		"chlorine",
		"argon",
		"potassium",
		"calcium",
		"scandium",
		"titanium",
		"vanadium",
		"chromium",
		"manganese",
		"iron",
		"cobalt",
		"nickel",
		"copper",
		"zinc",
		"gallium",
		"germanium",
		"arsenic",
		"selenium",
		"bromine",
		"krypton",
		"rubidium",
		"strontium",
		"yttrium",
		"zirconium",
		"niobium",
		"molybdenum",
		"ruthenium",
		"rhodium",
		"palladium",
		"silver",
		"cadmium",
		"indium",
		"tin",
		"antimony",
		"tellurium",
		"iodine",
		"xenon",
		"caesium",
		"barium",
		"lanthanum",
		"cerium",
		"praseodymium",
		"neodymium",
		"samarium",
		"europium",
		"gadolinium",
		"terbium",
		"dysprosium",
		"holmium",
		"erbium",
		"thulium",
		"ytterbium",
		"lutetium",
		"hafnium",
		"tantalum",
		"tungsten",
		"rhenium",
		"osmium",
		"iridium",
		"platinum",
		"gold",
		"mercury",
		"thallium",
		"lead",
		"bismuth",
		"thorium",
		"uranium",
	}
)

func pick[T any](s []T) T {
	return s[rand.Intn(len(s))] //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)
}

// GetRandomName generates a random name from the list of adjectives and surnames in this package
// formatted as "adjective_surname". For example 'focused_turing'. If retry is non-zero, a random
// integer between 0 and 10 will be added to the end of the name, e.g `focused_turing3`
func GetRandomName(retry int) string {
	name := pick(adjective[:]) + "_" + pick(colour[:]) + "_" + pick(element[:])
	if retry > 0 {
		name += strconv.Itoa(rand.Intn(10)) //nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand)
	}
	return name
}
