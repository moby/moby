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
	left = [...]string{"happy", "jolly", "dreamy", "sad", "angry", "pensive", "focused", "sleepy", "grave", "distracted", "determined", "stoic", "stupefied", "sharp", "agitated", "cocky", "tender", "goofy", "furious", "desperate", "hopeful", "compassionate", "silly", "lonely", "condescending", "naughty", "kickass", "drunk", "boring", "nostalgic", "ecstatic", "insane", "cranky", "mad", "jovial", "sick", "hungry", "thirsty", "elegant", "backstabbing", "clever", "trusting", "loving", "suspicious", "berserk", "high", "romantic", "prickly", "evil"}
	// Docker 0.7.x generates names from notable scientists and hackers.
	//
	// Ada Lovelace invented the first algorithm. http://en.wikipedia.org/wiki/Ada_Lovelace (thanks James Turnbull)
	// Alan Turing was a founding father of computer science. http://en.wikipedia.org/wiki/Alan_Turing.
	// Albert Einstein invented the general theory of relativity. http://en.wikipedia.org/wiki/Albert_Einstein
	// Ambroise Pare invented modern surgery. http://en.wikipedia.org/wiki/Ambroise_Par%C3%A9
	// Archimedes was a physicist, engineer and mathematician who invented too many things to list them here. http://en.wikipedia.org/wiki/Archimedes
	// Benjamin Franklin is famous for his experiments in electricity and the invention of the lightning rod.
	// Charles Babbage invented the concept of a programmable computer. http://en.wikipedia.org/wiki/Charles_Babbage.
	// Charles Darwin established the principles of natural evolution. http://en.wikipedia.org/wiki/Charles_Darwin.
	// Dennis Ritchie and Ken Thompson created UNIX and the C programming language. http://en.wikipedia.org/wiki/Dennis_Ritchie http://en.wikipedia.org/wiki/Ken_Thompson
	// Douglas Engelbart gave the mother of all demos: http://en.wikipedia.org/wiki/Douglas_Engelbart
	// Emmett Brown invented time travel. http://en.wikipedia.org/wiki/Emmett_Brown (thanks Brian Goff)
	// Enrico Fermi invented the first nuclear reactor. http://en.wikipedia.org/wiki/Enrico_Fermi.
	// Euclid invented geometry. http://en.wikipedia.org/wiki/Euclid
	// Galileo was a founding father of modern astronomy, and faced politics and obscurantism to establish scientific truth.  http://en.wikipedia.org/wiki/Galileo_Galilei
	// Henry Poincare made fundamental contributions in several fields of mathematics. http://en.wikipedia.org/wiki/Henri_Poincar%C3%A9
	// Isaac Newton invented classic mechanics and modern optics. http://en.wikipedia.org/wiki/Isaac_Newton
	// John McCarthy invented LISP: http://en.wikipedia.org/wiki/John_McCarthy_(computer_scientist)
	// Leonardo Da Vinci invented too many things to list here. http://en.wikipedia.org/wiki/Leonardo_da_Vinci.
	// Linus Torvalds invented Linux and Git. http://en.wikipedia.org/wiki/Linus_Torvalds
	// Louis Pasteur discovered vaccination, fermentation and pasteurization. http://en.wikipedia.org/wiki/Louis_Pasteur.
	// Malcolm McLean invented the modern shipping container: http://en.wikipedia.org/wiki/Malcom_McLean
	// Marie Curie discovered radioactivity. http://en.wikipedia.org/wiki/Marie_Curie.
	// Muhammad ibn Jābir al-Ḥarrānī al-Battānī was a founding father of astronomy. http://en.wikipedia.org/wiki/Mu%E1%B8%A5ammad_ibn_J%C4%81bir_al-%E1%B8%A4arr%C4%81n%C4%AB_al-Batt%C4%81n%C4%AB
	// Niels Bohr is the father of quantum theory. http://en.wikipedia.org/wiki/Niels_Bohr.
	// Nikola Tesla invented the AC electric system and every gaget ever used by a James Bond villain. http://en.wikipedia.org/wiki/Nikola_Tesla
	// Pierre de Fermat pioneered several aspects of modern mathematics. http://en.wikipedia.org/wiki/Pierre_de_Fermat
	// Richard Feynmann was a key contributor to quantum mechanics and particle physics. http://en.wikipedia.org/wiki/Richard_Feynman
	// Rob Pike was a key contributor to Unix, Plan 9, the X graphic system, utf-8, and the Go programming language. http://en.wikipedia.org/wiki/Rob_Pike
	// Stephen Hawking pioneered the field of cosmology by combining general relativity and quantum mechanics. http://en.wikipedia.org/wiki/Stephen_Hawking
	// Steve Wozniak invented the Apple I and Apple II. http://en.wikipedia.org/wiki/Steve_Wozniak
	// Werner Heisenberg was a founding father of quantum mechanics. http://en.wikipedia.org/wiki/Werner_Heisenberg
	// William Shockley, Walter Houser Brattain and John Bardeen co-invented the transistor (thanks Brian Goff).
	//	http://en.wikipedia.org/wiki/John_Bardeen
	//	http://en.wikipedia.org/wiki/Walter_Houser_Brattain
	//	http://en.wikipedia.org/wiki/William_Shockley
	right = [...]string{"lovelace", "franklin", "tesla", "einstein", "bohr", "davinci", "pasteur", "nobel", "curie", "darwin", "turing", "ritchie", "torvalds", "pike", "thompson", "wozniak", "galileo", "euclide", "newton", "fermat", "archimede", "poincare", "heisenberg", "feynmann", "hawkings", "fermi", "pare", "mccarthy", "engelbart", "babbage", "albattani", "ptolemy", "bell", "wright", "lumiere", "morse", "mclean", "brown", "bardeen", "brattain", "shockley"}
)

func GenerateRandomName(checker NameChecker) (string, error) {
	retry := 5
	rand.Seed(time.Now().UnixNano())
	name := fmt.Sprintf("%s_%s", left[rand.Intn(len(left))], right[rand.Intn(len(right))])
	for checker != nil && checker.Exists(name) && retry > 0 {
		name = fmt.Sprintf("%s%d", name, rand.Intn(10))
		retry = retry - 1
	}
	if retry == 0 {
		return name, fmt.Errorf("Error generating random name")
	}
	return name, nil
}
