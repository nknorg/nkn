package abi

type Event struct {
	Name      string
	Anonymous bool
	Inputs    []Argument
}