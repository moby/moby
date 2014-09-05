package namesgenerator

import (
	"fmt"
	"math/rand"
	"time"
)

var (
	left = [...]string{"happy", "jolly", "dreamy", "sad", "angry", "pensive", "focused", "sleepy", "grave", "distracted", "determined", "stoic", "stupefied", "sharp", "agitated", "cocky", "tender", "goofy", "furious", "desperate", "hopeful", "compassionate", "silly", "lonely", "condescending", "naughty", "kickass", "drunk", "boring", "nostalgic", "ecstatic", "insane", "cranky", "mad", "jovial", "sick", "hungry", "thirsty", "elegant", "backstabbing", "clever", "trusting", "loving", "suspicious", "berserk", "high", "romantic", "prickly", "evil"}
	// Docker 0.7.x generates names from notable scientists and hackers.
	//
	// Ada Lovelace invented the first algorithm. http://en.wikipedia.org/wiki/Ada_Lovelace (thanks James Turnbull)
	// Ada Yonath - an Israeli crystallographer, the first woman from the Middle East to win a Nobel prize in the sciences. http://en.wikipedia.org/wiki/Ada_Yonath
	// Adele Goldstine, born Adele Katz, wrote the complete technical description for the first electronic digital computer, ENIAC. http://en.wikipedia.org/wiki/Adele_Goldstine
	// Alan Turing was a founding father of computer science. http://en.wikipedia.org/wiki/Alan_Turing.
	// Albert Einstein invented the general theory of relativity. http://en.wikipedia.org/wiki/Albert_Einstein
	// Ambroise Pare invented modern surgery. http://en.wikipedia.org/wiki/Ambroise_Par%C3%A9
	// Archimedes was a physicist, engineer and mathematician who invented too many things to list them here. http://en.wikipedia.org/wiki/Archimedes
	// Barbara McClintock - a distinguished American cytogeneticist, 1983 Nobel Laureate in Physiology or Medicine for discovering transposons. http://en.wikipedia.org/wiki/Barbara_McClintock
	// Benjamin Franklin is famous for his experiments in electricity and the invention of the lightning rod.
	// Charles Babbage invented the concept of a programmable computer. http://en.wikipedia.org/wiki/Charles_Babbage.
	// Charles Darwin established the principles of natural evolution. http://en.wikipedia.org/wiki/Charles_Darwin.
	// Dennis Ritchie and Ken Thompson created UNIX and the C programming language. http://en.wikipedia.org/wiki/Dennis_Ritchie http://en.wikipedia.org/wiki/Ken_Thompson
	// Douglas Engelbart gave the mother of all demos: http://en.wikipedia.org/wiki/Douglas_Engelbart
	// Elizabeth Blackwell - American doctor and first American woman to receive a medical degree - http://en.wikipedia.org/wiki/Elizabeth_Blackwell
	// Emmett Brown invented time travel. http://en.wikipedia.org/wiki/Emmett_Brown (thanks Brian Goff)
	// Enrico Fermi invented the first nuclear reactor. http://en.wikipedia.org/wiki/Enrico_Fermi.
	// Erna Schneider Hoover revolutionized modern communication by inventing a computerized telephon switching method. http://en.wikipedia.org/wiki/Erna_Schneider_Hoover
	// Euclid invented geometry. http://en.wikipedia.org/wiki/Euclid
	// Françoise Barré-Sinoussi - French virologist and Nobel Prize Laureate in Physiology or Medicine; her work was fundamental in identifying HIV as the cause of AIDS. http://en.wikipedia.org/wiki/Fran%C3%A7oise_Barr%C3%A9-Sinoussi
	// Galileo was a founding father of modern astronomy, and faced politics and obscurantism to establish scientific truth.  http://en.wikipedia.org/wiki/Galileo_Galilei
	// Gertrude Elion - American biochemist, pharmacologist and the 1988 recipient of the Nobel Prize in Medicine - http://en.wikipedia.org/wiki/Gertrude_Elion
	// Grace Hopper developed the first compiler for a computer programming language and  is credited with popularizing the term "debugging" for fixing computer glitches. http://en.wikipedia.org/wiki/Grace_Hopper
	// Henry Poincare made fundamental contributions in several fields of mathematics. http://en.wikipedia.org/wiki/Henri_Poincar%C3%A9
	// Hypatia - Greek Alexandrine Neoplatonist philosopher in Egypt who was one of the earliest mothers of mathematics - http://en.wikipedia.org/wiki/Hypatia
	// Isaac Newton invented classic mechanics and modern optics. http://en.wikipedia.org/wiki/Isaac_Newton
	// Jane Colden - American botanist widely considered the first female American botanist - http://en.wikipedia.org/wiki/Jane_Colden
	// Jane Goodall - British primatologist, ethologist, and anthropologist who is considered to be the world's foremost expert on chimpanzees - http://en.wikipedia.org/wiki/Jane_Goodall
	// Jean Bartik, born Betty Jean Jennings, was one of the original programmers for the ENIAC computer. http://en.wikipedia.org/wiki/Jean_Bartik
	// Jean E. Sammet developed FORMAC, the first widely used computer language for symbolic manipulation of mathematical formulas. http://en.wikipedia.org/wiki/Jean_E._Sammet
	// Johanna Mestorf - German prehistoric archaeologist and first female museum director in Germany - http://en.wikipedia.org/wiki/Johanna_Mestorf
	// John McCarthy invented LISP: http://en.wikipedia.org/wiki/John_McCarthy_(computer_scientist)
	// June Almeida - Scottish virologist who took the first pictures of the rubella virus - http://en.wikipedia.org/wiki/June_Almeida
	// Karen Spärck Jones came up with the concept of inverse document frequency, which is used in most search engines today. http://en.wikipedia.org/wiki/Karen_Sp%C3%A4rck_Jones
	// Leonardo Da Vinci invented too many things to list here. http://en.wikipedia.org/wiki/Leonardo_da_Vinci.
	// Linus Torvalds invented Linux and Git. http://en.wikipedia.org/wiki/Linus_Torvalds
	// Lise Meitner - Austrian/Swedish physicist who was involved in the discovery of nuclear fission. The element meitnerium is named after her - http://en.wikipedia.org/wiki/Lise_Meitner
	// Louis Pasteur discovered vaccination, fermentation and pasteurization. http://en.wikipedia.org/wiki/Louis_Pasteur.
	// Malcolm McLean invented the modern shipping container: http://en.wikipedia.org/wiki/Malcom_McLean
	// Maria Ardinghelli - Italian translator, mathematician and physicist - http://en.wikipedia.org/wiki/Maria_Ardinghelli
	// Maria Kirch - German astronomer and first woman to discover a comet - http://en.wikipedia.org/wiki/Maria_Margarethe_Kirch
	// Maria Mayer - American theoretical physicist and Nobel laureate in Physics for proposing the nuclear shell model of the atomic nucleus - http://en.wikipedia.org/wiki/Maria_Mayer
	// Marie Curie discovered radioactivity. http://en.wikipedia.org/wiki/Marie_Curie.
	// Marie-Jeanne de Lalande - French astronomer, mathematician and cataloguer of stars - http://en.wikipedia.org/wiki/Marie-Jeanne_de_Lalande
	// Mary Leakey - British paleoanthropologist who discovered the first fossilized Proconsul skull - http://en.wikipedia.org/wiki/Mary_Leakey
	// Muhammad ibn Jābir al-Ḥarrānī al-Battānī was a founding father of astronomy. http://en.wikipedia.org/wiki/Mu%E1%B8%A5ammad_ibn_J%C4%81bir_al-%E1%B8%A4arr%C4%81n%C4%AB_al-Batt%C4%81n%C4%AB
	// Niels Bohr is the father of quantum theory. http://en.wikipedia.org/wiki/Niels_Bohr.
	// Nikola Tesla invented the AC electric system and every gaget ever used by a James Bond villain. http://en.wikipedia.org/wiki/Nikola_Tesla
	// Pierre de Fermat pioneered several aspects of modern mathematics. http://en.wikipedia.org/wiki/Pierre_de_Fermat
	// Rachel Carson - American marine biologist and conservationist, her book Silent Spring and other writings are credited with advancing the global environmental movement. http://en.wikipedia.org/wiki/Rachel_Carson
	// Radia Perlman is a software designer and network engineer and most famous for her invention of the spanning-tree protocol (STP). http://en.wikipedia.org/wiki/Radia_Perlman
	// Richard Feynman was a key contributor to quantum mechanics and particle physics. http://en.wikipedia.org/wiki/Richard_Feynman
	// Richard Matthew Stallman - the founder of the Free Software movement, the GNU project, the Free Software Foundation, and the League for Programming Freedom. He also invented the concept of copyleft to protect the ideals of this movement, and enshrined this concept in the widely-used GPL (General Public License) for software. http://en.wikiquote.org/wiki/Richard_Stallman
	// Rob Pike was a key contributor to Unix, Plan 9, the X graphic system, utf-8, and the Go programming language. http://en.wikipedia.org/wiki/Rob_Pike
	// Rosalind Franklin - British biophysicist and X-ray crystallographer whose research was critical to the understanding of DNA - http://en.wikipedia.org/wiki/Rosalind_Franklin
	// Sophie Kowalevski - Russian mathematician responsible for important original contributions to analysis, differential equations and mechanics - http://en.wikipedia.org/wiki/Sofia_Kovalevskaya
	// Sophie Wilson designed the first Acorn Micro-Computer and the instruction set for ARM processors. http://en.wikipedia.org/wiki/Sophie_Wilson
	// Stephen Hawking pioneered the field of cosmology by combining general relativity and quantum mechanics. http://en.wikipedia.org/wiki/Stephen_Hawking
	// Steve Wozniak invented the Apple I and Apple II. http://en.wikipedia.org/wiki/Steve_Wozniak
	// Werner Heisenberg was a founding father of quantum mechanics. http://en.wikipedia.org/wiki/Werner_Heisenberg
	// William Shockley, Walter Houser Brattain and John Bardeen co-invented the transistor (thanks Brian Goff).
	//	http://en.wikipedia.org/wiki/John_Bardeen
	//	http://en.wikipedia.org/wiki/Walter_Houser_Brattain
	//	http://en.wikipedia.org/wiki/William_Shockley
	right = [...]string{"albattani", "almeida", "archimedes", "ardinghelli", "babbage", "bardeen", "bartik", "bell", "blackwell", "bohr", "brattain", "brown", "carson", "colden", "curie", "darwin", "davinci", "einstein", "elion", "engelbart", "euclid", "fermat", "fermi", "feynman", "franklin", "galileo", "goldstine", "goodall", "hawking", "heisenberg", "hoover", "hopper", "hypatia", "jones", "kirch", "kowalevski", "lalande", "leakey", "lovelace", "lumiere", "mayer", "mccarthy", "mcclintock", "mclean", "meitner", "mestorf", "morse", "newton", "nobel", "pare", "pasteur", "perlman", "pike", "poincare", "ptolemy", "ritchie", "rosalind", "sammet", "shockley", "sinoussi", "stallman", "tesla", "thompson", "torvalds", "turing", "wilson", "wozniak", "wright", "yonath"}
)

func GetRandomName(retry int) string {
	rand.Seed(time.Now().UnixNano())

begin:
	name := fmt.Sprintf("%s_%s", left[rand.Intn(len(left))], right[rand.Intn(len(right))])
	if name == "boring_wozniak" /* Steve Wozniak is not boring */ {
		goto begin
	}

	if retry > 0 {
		name = fmt.Sprintf("%s%d", name, rand.Intn(10))
	}
	return name
}
