package types

type LangType byte

const (
	CSharp  LangType = iota
	Solidity
)

type VmType byte

const (
	AVM VmType = iota
	EVM
)

var (
	LangVm = map[LangType]VmType{
		CSharp: AVM,
		Solidity: EVM,
	}
)

type TriggerType byte

const (
	Verification TriggerType = iota
	Application
)
