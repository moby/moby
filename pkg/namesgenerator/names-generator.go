package namesgenerator

import (
	"fmt"
	"math/rand"
	"time"
)

type Person struct {
	Name, Description, Url, Thanks string
}

// Docker 0.7.x generates names from notable scientists and hackers.
//
var People = map[string]Person{
	"lovelace":    Person{"Ada Lovelace", "invented the first algorithm.", "http://en.wikipedia.org/wiki/Ada_Lovelace", "(thanks James Turnbull)"},
	"yonath":      Person{"Ada Yonath", "an Israeli crystallographer, the first woman from the Middle East to win a Nobel prize in the sciences.", "http://en.wikipedia.org/wiki/Ada_Yonath", ""},
	"goldstine":   Person{"Adele Goldstine", "born Adele Katz, wrote the complete technical description for the first electronic digital computer, ENIAC.", "http://en.wikipedia.org/wiki/Adele_Goldstine", ""},
	"turing":      Person{"Alan Turing", "was a founding father of computer science.", "http://en.wikipedia.org/wiki/Alan_Turing", ""},
	"einstein":    Person{"Albert Einstein", "invented the general theory of relativity.", "http://en.wikipedia.org/wiki/Albert_Einstein", ""},
	"pare":        Person{"Ambroise Pare", "invented modern surgery.", "http://en.wikipedia.org/wiki/Ambroise_Par%C3%A9", ""},
	"archimedes":  Person{"Archimedes", "was a physicist, engineer and mathematician who invented too many things to list them here.", "http://en.wikipedia.org/wiki/Archimedes", ""},
	"mcclintock":  Person{"Barbara McClintock", "- a distinguished American cytogeneticist, 1983 Nobel Laureate in Physiology or Medicine for discovering transposons.", "http://en.wikipedia.org/wiki/Barbara_McClintock", ""},
	"franklin":    Person{"Benjamin Franklin\nRosalind Franklin", "Benjamin Franklin is famous for his experiments in electricity and the invention of the lightning rod. \nRosalind Franklin British biophysicist and X-ray crystallographer whose research was critical to the understanding of DNA", "http://en.wikipedia.org/wiki/Benjamin_Franklin\nhttp://en.wikipedia.org/wiki/Rosalind_Franklin", ""},
	"babbage":     Person{"Charles Babbage", "invented the concept of a programmable computer.", "http://en.wikipedia.org/wiki/Charles_Babbage", ""},
	"darwin":      Person{"Charles Darwin", "established the principles of natural evolution.", "http://en.wikipedia.org/wiki/Charles_Darwin", ""},
	"ritchie":     Person{"Dennis Ritchie", "With Ken Thompson created UNIX and the C programming language.", "http://en.wikipedia.org/wiki/Dennis_Ritchie", ""},
	"thompson":    Person{"Ken Thompsoni", "With Dennis Ritchie created UNIX and the C programming language.", "http://en.wikipedia.org/wiki/Ken_Thompson", ""},
	"hodgkin":     Person{"Dorothy Hodgkin", "was a British biochemist credited with the development of protein crystallography. She was awarded the Nobel Prize in Chemistry in 1964.", "http://en.wikipedia.org/wiki/Dorothy_Hodgkin", ""},
	"engelbart":   Person{"Douglas Engelbart", "gave the mother of all demos", "http://en.wikipedia.org/wiki/Douglas_Engelbart", ""},
	"blackwell":   Person{"Elizabeth Blackwell", "American doctor and first American woman to receive a medical degree", "http://en.wikipedia.org/wiki/Elizabeth_Blackwell", ""},
	"brown":       Person{"Emmett Brown", "invented time travel", "http://en.wikipedia.org/wiki/Emmett_Brown", "(thanks Brian Goff)"},
	"fermi":       Person{"Enrico Fermi", "invented the first nuclear reactor.", "http://en.wikipedia.org/wiki/Enrico_Fermi. ", ""},
	"hoover":      Person{"Erna Schneider Hoover", "revolutionized modern communication by inventing a computerized telephon switching method.", "http://en.wikipedia.org/wiki/Erna_Schneider_Hoover", ""},
	"euclid":      Person{"Euclid invented geometry", ".", "http://en.wikipedia.org/wiki/Euclid", ""},
	"sinoussi":    Person{"Françoise Barré-Sinoussi", "French virologist and Nobel Prize Laureate in Physiology or Medicine; her work was fundamental in identifying HIV as the cause of AIDS.", "http://en.wikipedia.org/wiki/Fran%C3%A7oise_Barr%C3%A9-Sinoussi", ""},
	"galileo":     Person{"Galileo", "was a founding father of modern astronomy, and faced politics and obscurantism to establish scientific truth.", "http://en.wikipedia.org/wiki/Galileo_Galilei", ""},
	"elion":       Person{"Gertrude Elion ", "American biochemist, pharmacologist and the 1988 recipient of the Nobel Prize in Medicine", "http://en.wikipedia.org/wiki/Gertrude_Elion", ""},
	"cori":        Person{"Gerty Theresa Cori ", "American biochemist who became the third woman, —and first American woman—to win a Nobel Prize in science, and the first woman to be awarded the Nobel Prize in Physiology or Medicine. Cori was born in Prague.", "http://en.wikipedia.org/wiki/Gerty_Cori", ""},
	"hopper":      Person{"Grace Hopper", "developed the first compiler for a computer programming language and  is credited with popularizing the term, debugging for fixing computer glitches.", "http://en.wikipedia.org/wiki/Grace_Hopper", ""},
	"poincare":    Person{"Henry Poincare", "made fundamental contributions in several fields of mathematics.", "http://en.wikipedia.org/wiki/Henri_Poincar%C3%A9", ""},
	"hypatia":     Person{"Hypatia", "Greek Alexandrine Neoplatonist philosopher in Egypt who was one of the earliest mothers of mathematics", "http://en.wikipedia.org/wiki/Hypatia", ""},
	"newton":      Person{"Isaac Newton", "invented classic mechanics and modern optics", "http://en.wikipedia.org/wiki/Isaac_Newton", ""},
	"golden":      Person{"Jane Colden ", "American botanist widely considered the first female American botanist", "http://en.wikipedia.org/wiki/Jane_Colden", ""},
	"goodall":     Person{"Jane Goodall", "British primatologist, ethologist, and anthropologist who is considered to be the world's foremost expert on chimpanzees", "http://en.wikipedia.org/wiki/Jane_Goodall", ""},
	"bartik":      Person{"Jean Bartik", "born Betty Jean Jennings, was one of the original programmers for the ENIAC computer", "http://en.wikipedia.org/wiki/Jean_Bartik", ""},
	"sammet":      Person{"Jean E", ". Sammet developed FORMAC, the first widely used computer language for symbolic manipulation of mathematical formulas.", "http://en.wikipedia.org/wiki/Jean_E._Sammet", ""},
	"mestorf":     Person{"Johanna Mestorf ", "German prehistoric archaeologist and first female museum director in Germany", "http://en.wikipedia.org/wiki/Johanna_Mestorf", ""},
	"mccarty":     Person{"John McCarthy", "invented LISP", "http://en.wikipedia.org/wiki/John_McCarthy_(computer_scientist)", ""},
	"almeida":     Person{"June Almeida", "Scottish virologist who took the first pictures of the rubella virus", "http://en.wikipedia.org/wiki/June_Almeida", ""},
	"jones":       Person{"Karen Spärck Jones", "came up with the concept of inverse document frequency, which is used in most search engines today.", "http://en.wikipedia.org/wiki/Karen_Sp%C3%A4rck_Jones", ""},
	"davinci":     Person{"Leonardo Da Vinci invented too many things to list here", ".", "http://en.wikipedia.org/wiki/Leonardo_da_Vinci", ""},
	"torvalds":    Person{"Linus Torvalds", "invented Linux and Git.", "http://en.wikipedia.org/wiki/Linus_Torvalds", ""},
	"meitner":     Person{"Lise Meitner", "Austrian/Swedish physicist who was involved in the discovery of nuclear fission. The element meitnerium is named after her", "http://en.wikipedia.org/wiki/Lise_Meitner", ""},
	"pasteur":     Person{"Louis Pasteur", "discovered vaccination, fermentation and pasteurization", "http://en.wikipedia.org/wiki/Louis_Pasteur", ""},
	"mclean":      Person{"Malcolm McLean", "invented the modern shipping container", "http://en.wikipedia.org/wiki/Malcom_McLean", ""},
	"ardinghelli": Person{"Maria Ardinghelli", "Italian translator, mathematician and physicist", "http://en.wikipedia.org/wiki/Maria_Ardinghelli", ""},
	"kirch":       Person{"Maria Kirch", "German astronomer and first woman to discover a comet", "http://en.wikipedia.org/wiki/Maria_Margarethe_Kirch", ""},
	"mayer":       Person{"Maria Mayer", "American theoretical physicist and Nobel laureate in Physics for proposing the nuclear shell model of the atomic nucleus", "http://en.wikipedia.org/wiki/Maria_Mayer", ""},
	"curie":       Person{"Marie Curie", "discovered radioactivity. http", "://en.wikipedia.org/wiki/Marie_Curie", ""},
	"lalande":     Person{"Marie-Jeanne de Lalande", "French astronomer, mathematician and cataloguer of stars", "http://en.wikipedia.org/wiki/Marie-Jeanne_de_Lalande", ""},
	"leakey":      Person{"Mary Leakey", "British paleoanthropologist who discovered the first fossilized Proconsul skull", "http://en.wikipedia.org/wiki/Mary_Leakey", ""},
	"albattani":   Person{"Muhammad ibn Jābir al-Ḥarrānī al-Battānī", "was a founding father of astronomy.", "http://en.wikipedia.org/wiki/Mu%E1%B8%A5ammad_ibn_J%C4%81bir_al-%E1%B8%A4arr%C4%81n%C4%AB_al-Batt%C4%81n%C4%AB", ""},
	"bohr":        Person{"Niels Bohr", "is the father of quantum theory.", "http://en.wikipedia.org/wiki/Niels_Bohr", ""},
	"tesla":       Person{"Nikola Tesla", "invented the AC electric system and every gadget ever used by a James Bond villain.", "http://en.wikipedia.org/wiki/Nikola_Tesla", ""},
	"fermat":      Person{"Pierre de Fermat", "pioneered several aspects of modern mathematics.", "http://en.wikipedia.org/wiki/Pierre_de_Fermat", ""},
	"carson":      Person{"Rachel Carson", "American marine biologist and conservationist, her book Silent Spring and other writings are credited with advancing the global environmental movement.", "http://en.wikipedia.org/wiki/Rachel_Carson", ""},
	"perlman":     Person{"Radia Perlman", "is a software designer and network engineer and most famous for her invention of the spanning-tree protocol (STP).", "http://en.wikipedia.org/wiki/Radia_Perlman", ""},
	"feynman":     Person{"Richard Feynman", "was a key contributor to quantum mechanics and particle physics.", "http://en.wikipedia.org/wiki/Richard_Feynman", ""},
	"stallman":    Person{"Richard Matthew Stallman", "the founder of the Free Software movement, the GNU project, the Free Software Foundation, and the League for Programming Freedom. He also invented the concept of copyleft to protect the ideals of this movement, and enshrined this concept in the widely-used GPL (General Public License) for software", "http://en.wikiquote.org/wiki/Richard_Stallman", ""},
	"pike":        Person{"Rob Pike", "was a key contributor to Unix, Plan 9, the X graphic system, utf-8, and the Go programming language.", "http://en.wikipedia.org/wiki/Rob_Pike", ""},
	"yalow":       Person{"Rosalyn Sussman Yalow", "Rosalyn Sussman Yalow was an American medical physicist, and a co-winner of the 1977 Nobel Prize in Physiology or Medicine for development of the radioimmunoassay technique.", "http://en.wikipedia.org/wiki/Rosalyn_Sussman_Yalow", ""},
	"kowalevski":  Person{"Sophie Kowalevski", "Russian mathematician responsible for important original contributions to analysis, differential equations and mechanics", "http://en.wikipedia.org/wiki/Sofia_Kovalevskaya", ""},
	"wilson":      Person{"Sophie Wilson", "designed the first Acorn Micro Computer and the instruction set for ARM processors.", "http://en.wikipedia.org/wiki/Sophie_Wilson", ""},
	"hawking":     Person{"Stephen Hawking", "pioneered the field of cosmology by combining general relativity and quantum mechanics", "http://en.wikipedia.org/wiki/Stephen_Hawkinig", ""},
	"wozniak":     Person{"Steve Wozniak", "invented the Apple I and Apple II", ". http://en.wikipedia.org/wiki/Steve_Wozniak", ""},
	"heisenberg":  Person{"Werner Heisenberg", "was a founding father of quantum mechanics", "http://en.wikipedia.org/wiki/Werner_Heisenberg", ""},
	"shockley":    Person{"William Shockley", "with Walter Houser Brattain and John Bardeen co-invented the transistor", "http://en.wikipedia.org/wiki/John_Bardee", "(thanks Brian Goff)"},
	"brattain":    Person{"Walter Houser Brattain", "With William Shockley and John Bardeen co-invented the transistor", "http://en.wikipedia.org/wiki/Walter_Houser_Brattain", "(thanks Brian Goff)"},
	"bardeen":     Person{"John Bardeen", "With William Shockley and Walter Houser Brattain co-invented the transistor", "http://en.wikipedia.org/wiki/William_Shockley", "(thanks Brian Goff)."},
	"jang":        Person{"Yeong-Sil Jang", "was a Korean scientist and astronomer during the Joseon Dynasty, he invented the first metal printing press and water gauge.", "http://en.wikipedia.org/wiki/Jang_Yeong-sil", ""},
}

var (
	left  = [...]string{"happy", "jolly", "dreamy", "sad", "angry", "pensive", "focused", "sleepy", "grave", "distracted", "determined", "stoic", "stupefied", "sharp", "agitated", "cocky", "tender", "goofy", "furious", "desperate", "hopeful", "compassionate", "silly", "lonely", "condescending", "naughty", "kickass", "drunk", "boring", "nostalgic", "ecstatic", "insane", "cranky", "mad", "jovial", "sick", "hungry", "thirsty", "elegant", "backstabbing", "clever", "trusting", "loving", "suspicious", "berserk", "high", "romantic", "prickly", "evil", "admiring", "adoring", "reverent", "serene", "fervent", "modest", "gloomy", "elated"}
	right = [...]string{"albattani", "almeida", "archimedes", "ardinghelli", "babbage", "bardeen", "bartik", "bell", "blackwell", "bohr", "brattain", "brown", "carson", "colden", "cori", "curie", "darwin", "davinci", "einstein", "elion", "engelbart", "euclid", "fermat", "fermi", "feynman", "franklin", "galileo", "goldstine", "goodall", "hawking", "heisenberg", "hodgkin", "hoover", "hopper", "hypatia", "jang", "jones", "kirch", "kowalevski", "lalande", "leakey", "lovelace", "lumiere", "mayer", "mccarthy", "mcclintock", "mclean", "meitner", "mestorf", "morse", "newton", "nobel", "pare", "pasteur", "perlman", "pike", "poincare", "ptolemy", "ritchie", "rosalind", "sammet", "shockley", "sinoussi", "stallman", "tesla", "thompson", "torvalds", "turing", "wilson", "wozniak", "wright", "yalow", "yonath"}
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
