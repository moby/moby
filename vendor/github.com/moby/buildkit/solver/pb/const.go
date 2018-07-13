package pb

type InputIndex int64
type OutputIndex int64

const RootMount = "/"
const SkipOutput OutputIndex = -1
const Empty InputIndex = -1
const LLBBuilder InputIndex = -1

const LLBDefinitionInput = "buildkit.llb.definition"
const LLBDefaultDefinitionFile = LLBDefinitionInput
