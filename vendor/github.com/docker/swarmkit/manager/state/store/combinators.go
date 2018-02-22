package store

type orCombinator struct {
	bys []By
}

func (b orCombinator) isBy() {
}

// Or returns a combinator that applies OR logic on all the supplied By
// arguments.
func Or(bys ...By) By {
	return orCombinator{bys: bys}
}
