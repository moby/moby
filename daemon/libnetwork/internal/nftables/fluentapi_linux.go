package nftables

type chainBuilder struct {
	chain string
	tm    *Modifier
}

func (cd BaseChain) Builder() chainBuilder {
	tm := &Modifier{}
	tm.create(cd, 1)
	return chainBuilder{chain: cd.Name, tm: tm}
}

func (cd Chain) Builder() chainBuilder {
	tm := &Modifier{}
	tm.create(cd, 1)
	return chainBuilder{chain: cd.Name, tm: tm}
}

func (b chainBuilder) Rule(rule ...string) chainBuilder {
	b.tm.create(Rule{
		Chain: b.chain,
		Rule:  rule,
	}, 1)
	return b
}

func (b chainBuilder) Create(tm *Modifier) {
	tm.cmds = append(tm.cmds, b.tm.cmds...)
}
