package nftables

type chainBuilder struct {
	chain string
	tm    *Modifier
}

func (b BaseChain) Builder() chainBuilder {
	tm := &Modifier{}
	tm.create(b, 1)
	return chainBuilder{chain: b.Name, tm: tm}
}

func (c Chain) Builder() chainBuilder {
	tm := &Modifier{}
	tm.create(c, 1)
	return chainBuilder{chain: c.Name, tm: tm}
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
